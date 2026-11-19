for (const list of document.querySelectorAll("[data-filter-list]")) {
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
