(() => {
  const WIDTH = 1200
  const HEIGHT = 675

  function draw(canvas, image, zoom, positionX, positionY) {
    const context = canvas.getContext('2d')
    const scale = Math.max(WIDTH / image.naturalWidth, HEIGHT / image.naturalHeight) * zoom
    const width = image.naturalWidth * scale
    const height = image.naturalHeight * scale
    context.clearRect(0, 0, WIDTH, HEIGHT)
    context.drawImage(image, (WIDTH - width) * positionX / 100, (HEIGHT - height) * positionY / 100, width, height)
  }

  function open(file) {
    if (!file) return Promise.resolve(null)
    return new Promise((resolve) => {
      const objectURL = URL.createObjectURL(file)
      const overlay = document.createElement('div')
      overlay.className = 'pmp-crop-overlay'
      overlay.innerHTML = `<section class="pmp-crop-dialog" role="dialog" aria-modal="true" aria-labelledby="pmp-crop-title"><header><div><small>ОБЛОЖКА РЕШЕНИЯ</small><h2 id="pmp-crop-title">Настройте кадрирование</h2><p>Выберите область, которая будет видна в каталоге. Итоговый формат — 1200 × 675.</p></div><button type="button" data-close aria-label="Закрыть">×</button></header><div class="pmp-crop-preview"><canvas width="${WIDTH}" height="${HEIGHT}"></canvas></div><div class="pmp-crop-controls"><label><span>По горизонтали</span><input name="position_x" type="range" min="0" max="100" value="50"></label><label><span>По вертикали</span><input name="position_y" type="range" min="0" max="100" value="50"></label><label><span>Масштаб</span><input name="zoom" type="range" min="1" max="2.5" step="0.01" value="1"></label></div><p class="pmp-crop-error" hidden></p><footer><button type="button" class="secondary" data-close>Отмена</button><button type="button" class="primary" data-confirm disabled>Использовать обложку</button></footer></section>`
      const canvas = overlay.querySelector('canvas')
      const image = new Image()
      const previousOverflow = document.body.style.overflow
      let finished = false

      function finish(value) {
        if (finished) return
        finished = true
        URL.revokeObjectURL(objectURL)
        document.body.style.overflow = previousOverflow
        document.removeEventListener('keydown', keydown)
        overlay.remove()
        resolve(value)
      }
      function redraw() {
        draw(canvas, image, Number(overlay.querySelector('[name=zoom]').value), Number(overlay.querySelector('[name=position_x]').value), Number(overlay.querySelector('[name=position_y]').value))
      }
      function keydown(event) { if (event.key === 'Escape') finish(null) }

      overlay.querySelectorAll('input[type=range]').forEach(input => input.addEventListener('input', redraw))
      overlay.querySelectorAll('[data-close]').forEach(button => button.addEventListener('click', () => finish(null)))
      overlay.addEventListener('mousedown', event => { if (event.target === overlay) finish(null) })
      overlay.querySelector('[data-confirm]').addEventListener('click', () => {
        const confirm = overlay.querySelector('[data-confirm]')
        confirm.disabled = true
        confirm.textContent = 'Подготавливаем…'
        redraw()
        canvas.toBlob(blob => {
          if (!blob) {
            const error = overlay.querySelector('.pmp-crop-error')
            error.hidden = false
            error.textContent = 'Не удалось подготовить обложку'
            confirm.disabled = false
            confirm.textContent = 'Использовать обложку'
            return
          }
          const baseName = file.name.replace(/\.[^.]+$/, '') || 'cover'
          finish(new File([blob], `${baseName}-1200x675.webp`, { type: 'image/webp' }))
        }, 'image/webp', 0.9)
      })
      image.onload = () => { redraw(); overlay.querySelector('[data-confirm]').disabled = false }
      image.onerror = () => finish(null)
      image.src = objectURL
      document.addEventListener('keydown', keydown)
      document.body.style.overflow = 'hidden'
      document.body.append(overlay)
    })
  }

  window.ProfiMarketCoverCropper = { open }
})()
