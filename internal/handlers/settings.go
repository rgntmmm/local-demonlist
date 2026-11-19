package handlers

import (
	"bytes"
	"errors"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"local-demonlist/internal/middleware"
)

func avatarPNG(data []byte) ([]byte, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 2048 || cfg.Height > 2048 {
		return nil, errors.New("Use a PNG or JPEG image up to 2048 × 2048 pixels.")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("Could not read this image.")
	}
	var out bytes.Buffer
	if err = png.Encode(&out, img); err != nil {
		return nil, err
	}
	if out.Len() > 5<<20 {
		return nil, errors.New("The processed avatar exceeds 5 MB.")
	}
	return out.Bytes(), nil
}

func RegisterSettings(r *gin.Engine, pool *pgxpool.Pool) {
	r.GET("/players/:id/avatar", func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			c.Status(404)
			return
		}
		var data []byte
		err = pool.QueryRow(c.Request.Context(), "SELECT avatar_data FROM players WHERE id=$1 AND is_active", id).Scan(&data)
		if errors.Is(err, pgx.ErrNoRows) {
			c.Status(404)
			return
		}
		if err != nil {
			pageError(c, err)
			return
		}
		if len(data) == 0 {
			c.Redirect(http.StatusFound, "/static/images/default_avatar.png")
			return
		}
		c.Header("X-Content-Type-Options", "nosniff")
		c.Data(http.StatusOK, "image/png", data)
	})
	show := func(c *gin.Context, status int, message string) {
		var name string
		if err := pool.QueryRow(c.Request.Context(), "SELECT display_name FROM players WHERE id=$1", c.GetInt64("accountID")).Scan(&name); err != nil {
			pageError(c, err)
			return
		}
		renderPage(c, status, "settings.html", gin.H{"DisplayName": name, "Error": message, "Saved": c.Query("saved")})
	}
	group := r.Group("/settings", middleware.RequireSession())
	group.GET("", func(c *gin.Context) { show(c, 200, "") })
	group.POST("/avatar", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, (5<<20)+(64<<10))
		if err := c.Request.ParseMultipartForm(1 << 20); err != nil {
			if c.Request.MultipartForm != nil {
				c.Request.MultipartForm.RemoveAll()
			}
			show(c, 400, "Upload a PNG or JPEG file up to 5 MB.")
			return
		}
		defer c.Request.MultipartForm.RemoveAll()
		file, _, err := c.Request.FormFile("avatar")
		if err != nil {
			show(c, 400, "Choose an avatar image.")
			return
		}
		defer file.Close()
		raw, err := io.ReadAll(io.LimitReader(file, (5<<20)+1))
		if err != nil || len(raw) > 5<<20 {
			show(c, 400, "Avatar must be no larger than 5 MB.")
			return
		}
		data, err := avatarPNG(raw)
		if err != nil {
			show(c, 400, err.Error())
			return
		}
		if _, err = pool.Exec(c.Request.Context(), "UPDATE players SET avatar_data=$1 WHERE id=$2", data, c.GetInt64("accountID")); err != nil {
			pageError(c, err)
			return
		}
		c.Redirect(303, "/settings?saved=avatar")
	})
	group.POST("/nickname", func(c *gin.Context) {
		middleware.LimitRequestBody(c, 4096)
		if err := c.Request.ParseForm(); err != nil {
			show(c, 400, "Could not read the form.")
			return
		}
		name := strings.TrimSpace(c.PostForm("display_name"))
		if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 64 || strings.ContainsRune(name, 0) {
			show(c, 400, "Nickname must contain 1–64 characters.")
			return
		}
		ctx := c.Request.Context()
		tx, err := pool.Begin(ctx)
		if err != nil {
			pageError(c, err)
			return
		}
		defer tx.Rollback(ctx)
		// Match the lock order used by level management and verifier resolution.
		if _, err = tx.Exec(ctx, "LOCK TABLE levels, players IN SHARE ROW EXCLUSIVE MODE"); err != nil {
			pageError(c, err)
			return
		}
		id := c.GetInt64("accountID")
		var taken bool
		err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM players WHERE id<>$1 AND (lower(trim(display_name))=lower($2) OR lower(login)=lower($2)))", id, name).Scan(&taken)
		if err != nil {
			pageError(c, err)
			return
		}
		if taken {
			tx.Rollback(ctx)
			show(c, 409, "This nickname is already used by another account.")
			return
		}
		if _, err = tx.Exec(ctx, "UPDATE players SET display_name=$1 WHERE id=$2", name, id); err != nil {
			pageError(c, err)
			return
		}
		if _, err = tx.Exec(ctx, "UPDATE levels SET verifier=$1 WHERE verifier_player_id=$2", name, id); err != nil {
			pageError(c, err)
			return
		}
		if err = tx.Commit(ctx); err != nil {
			pageError(c, err)
			return
		}
		c.Redirect(303, "/settings?saved=nickname")
	})
	group.POST("/password", func(c *gin.Context) {
		middleware.LimitRequestBody(c, 4096)
		if err := c.Request.ParseForm(); err != nil {
			show(c, 400, "Could not read the form.")
			return
		}
		old, newPassword := c.PostForm("current_password"), c.PostForm("new_password")
		if len(old) > 128 || !utf8.ValidString(newPassword) || utf8.RuneCountInString(newPassword) < 8 || len(newPassword) > 72 {
			show(c, 400, "New password must contain at least 8 characters and at most 72 bytes.")
			return
		}
		if newPassword != c.PostForm("confirm_password") {
			show(c, 400, "New passwords do not match.")
			return
		}
		ctx := c.Request.Context()
		id := c.GetInt64("accountID")
		var stored string
		if err := pool.QueryRow(ctx, "SELECT password_hash FROM players WHERE id=$1", id).Scan(&stored); err != nil {
			pageError(c, err)
			return
		}
		if !passwordMatches(stored, old) {
			show(c, 400, "Current password is incorrect.")
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			pageError(c, err)
			return
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			pageError(c, err)
			return
		}
		defer tx.Rollback(ctx)
		tag, err := tx.Exec(ctx, "UPDATE players SET password_hash=$1 WHERE id=$2 AND password_hash=$3", string(hash), id, stored)
		if err != nil {
			pageError(c, err)
			return
		}
		if tag.RowsAffected() != 1 {
			tx.Rollback(ctx)
			show(c, 409, "Password changed in another session. Try again.")
			return
		}
		if _, err = tx.Exec(ctx, "DELETE FROM player_sessions WHERE player_id=$1", id); err != nil {
			pageError(c, err)
			return
		}
		if err = tx.Commit(ctx); err != nil {
			pageError(c, err)
			return
		}
		middleware.WriteSessionCookie(c, "", -1)
		c.Redirect(303, "/login?password_changed=1")
	})
}
