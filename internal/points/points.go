package points

const MaxPointsPos = 30

var PositionPoints = [MaxPointsPos + 1]int{
	0, 1000, 950, 900, 850, 800, 750, 700, 650, 600, 575,
	550, 525, 500, 475, 450, 425, 400, 375, 350, 325,
	300, 275, 250, 225, 200, 175, 150, 125, 100, 75,
}

func setPoints(levelPos int) int {
	if levelPos < 1 || levelPos > MaxPointsPos {
		return 0
	}

	return PositionPoints[levelPos]
}