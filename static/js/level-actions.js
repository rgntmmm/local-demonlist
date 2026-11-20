const deleteToggle = document.querySelector('button[aria-controls="delete-confirmation"]');
const confirmation = document.getElementById("delete-confirmation");

deleteToggle.addEventListener("click", () => {
    confirmation.hidden = !confirmation.hidden;
    deleteToggle.setAttribute("aria-expanded", String(!confirmation.hidden));
});
