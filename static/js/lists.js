function enableFilters(list) {
    const controls = list.querySelector("[data-filter-controls]");
    const search = controls.querySelector("input[type=search]");
    const difficulty = controls.querySelector("select");
    const compact = controls.querySelector("input[type=checkbox]");
    const status = controls.querySelector("[role=status]");
    const entries = Array.from(list.querySelectorAll("[data-filter-text]"), (element) => ({
        element,
        text: element.dataset.filterText.toLowerCase(),
    }));

    function updateList() {
        const query = search.value.trim().toLowerCase();
        const selectedDifficulty = difficulty?.value;
        let visibleCount = 0;

        for (const { element, text } of entries) {
            const matchesDifficulty = !selectedDifficulty ||
                element.dataset.difficulty === selectedDifficulty;
            const matches = text.includes(query) && matchesDifficulty;
            element.hidden = !matches;
            if (matches) visibleCount++;
        }

        list.classList.toggle("compact-list", compact?.checked === true);
        status.textContent = visibleCount
            ? `Showing ${visibleCount} of ${entries.length}`
            : "No matching results.";
    }

    search.addEventListener("input", updateList);
    difficulty?.addEventListener("change", updateList);
    compact?.addEventListener("change", updateList);
    controls.hidden = false;
    updateList();
}

const levelsCacheKey = "demonlist:levels:v1";
const levelsCacheTTL = 20 * 60 * 1000;

async function loadLevels(version) {
    const now = Date.now();
    try {
        const cached = JSON.parse(localStorage.getItem(levelsCacheKey));
        if (cached?.version === version && cached.savedAt <= now &&
            now - cached.savedAt < levelsCacheTTL && validLevels(cached.levels)) {
            return cached.levels;
        }
    } catch {
        // Storage can be disabled or contain an incomplete entry.
    }
    const response = await fetch("/api/levels", { cache: "no-store" });
    if (!response.ok) throw new Error("Could not load levels");
    const levels = await response.json();
    if (!validLevels(levels)) throw new Error("Invalid level list");
    try {
        localStorage.setItem(levelsCacheKey, JSON.stringify({ version, savedAt: now, levels }));
    } catch {
        // The current page can still display the response when storage is full.
    }
    return levels;
}

function validLevels(levels) {
    return Array.isArray(levels) && levels.every(level => level &&
        Number.isInteger(level.ID) && Number.isInteger(level.Position) &&
        Number.isInteger(level.Points) &&
        ["Name", "Author", "Difficulty", "Verifier", "Icon"].every(key => typeof level[key] === "string"));
}

function levelCard(template, level) {
    const card = template.content.firstElementChild.cloneNode(true);
    card.value = level.Position;
    card.dataset.filterText = level.Name + " " + level.Author;
    card.dataset.difficulty = level.Difficulty;
    card.querySelector(".position").textContent = "#" + level.Position;
    const icon = card.querySelector("img");
    if (/^https?:\/\//i.test(level.Icon)) icon.src = level.Icon;
    else icon.remove();

    const link = card.querySelector("h2 a");
    link.href = "/levels/" + level.ID;
    link.textContent = level.Name;
    const [creator, verifier] = card.querySelectorAll(".level-info p");
    creator.textContent = "Creator: " + level.Author;
    verifier.textContent = "Verifier: " + level.Verifier;
    const difficulty = level.Difficulty + " Demon";
    const points = level.Points + " points";
    link.title = [creator.textContent, level.Verifier && verifier.textContent, difficulty, points]
        .filter(Boolean).join(" · ");
    if (!level.Verifier) verifier.remove();
    card.querySelector(".badge").textContent = difficulty;
    card.querySelector(".points").textContent = points;
    return card;
}

async function showLevels(list) {
    const items = list.querySelector(".level-list");
    const status = items.firstElementChild;
    try {
        const levels = await loadLevels(list.dataset.levelVersion);
        const query = list.dataset.query.toLowerCase();
        const matches = levels.filter(level => level.Name.toLowerCase().includes(query) ||
            level.Author.toLowerCase().includes(query));
        if (matches.length) {
            const template = list.querySelector("#level-card");
            const cards = document.createDocumentFragment();
            for (const level of matches) cards.append(levelCard(template, level));
            items.replaceChildren(cards);
            enableFilters(list);
        } else {
            status.textContent = query ? "No matching levels. Try another search." : "No levels have been added yet.";
        }
        list.querySelector("[data-server-list]").hidden = true;
    } catch {
        status.textContent = "Could not load levels.";
    }
}

for (const list of document.querySelectorAll("[data-filter-list]")) {
    if (list.hasAttribute("data-level-version")) showLevels(list);
    else enableFilters(list);
}
