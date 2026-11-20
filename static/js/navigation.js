(() => {
    const drawer = document.querySelector('#mobile-menu');
    if (!drawer || typeof drawer.showModal !== 'function') return;

    const toggle = document.querySelector('.menu-toggle');
    const links = document.querySelector('.header-links');
    const header = toggle.parentElement;
    const logo = header.querySelector('.logo');
    header.classList.add('single-row');

    function updateLayout() {
        // Measure the full desktop row even while the real links are in the drawer.
        const measure = document.createElement('div');
        measure.className = 'header-inner single-row header-measure';
        measure.inert = true;
        measure.append(logo.cloneNode(true), links.cloneNode(true));
        header.after(measure);
        const fits = measure.getBoundingClientRect().width <= header.getBoundingClientRect().width;
        measure.remove();

        toggle.hidden = fits;
        if (!fits && links.parentElement !== drawer) {
            drawer.append(links);
        } else if (fits && links.parentElement === drawer) {
            if (drawer.open) drawer.close();
            drawer.before(links);
        }
    }

    toggle.addEventListener('click', () => {
        drawer.showModal();
        toggle.setAttribute('aria-expanded', 'true');
    });
    drawer.querySelector('.drawer-close').addEventListener('click', () => drawer.close());
    drawer.addEventListener('click', event => {
        if (event.target === drawer && event.clientX >= drawer.getBoundingClientRect().right) drawer.close();
    });
    drawer.addEventListener('close', () => toggle.setAttribute('aria-expanded', 'false'));
    new ResizeObserver(updateLayout).observe(header);
    document.fonts.addEventListener('loadingdone', updateLayout);
    updateLayout();
})();
