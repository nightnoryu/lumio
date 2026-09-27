(() => {
  const dialog = document.querySelector('#photo-viewer');
  const photos = [...document.querySelectorAll('[data-photo]')];
  if (!dialog || !dialog.showModal || !photos.length) return;
  const image = dialog.querySelector('[data-viewer-image]');
  let index = 0;
  let opener;
  function show(next) {
    index = (next + photos.length) % photos.length;
    const selected = photos[index].querySelector('img');
    image.src = photos[index].href;
    image.alt = selected.alt;
    dialog.querySelector('[data-caption]').textContent = selected.alt;
    dialog.querySelector('[data-position]').textContent = `${index + 1} / ${photos.length}`;
  }
  photos.forEach((photo, position) => photo.addEventListener('click', event => {
    if (event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;
    event.preventDefault();
    opener = photo;
    show(position);
    dialog.showModal();
    dialog.querySelector('[data-close]').focus();
  }));
  dialog.querySelector('[data-close]').addEventListener('click', () => dialog.close());
  dialog.querySelector('[data-previous]').addEventListener('click', () => show(index - 1));
  dialog.querySelector('[data-next]').addEventListener('click', () => show(index + 1));
  dialog.addEventListener('keydown', event => {
    if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
      event.preventDefault();
      show(index + (event.key === 'ArrowLeft' ? -1 : 1));
    }
  });
  dialog.addEventListener('close', () => opener?.focus());
})();
