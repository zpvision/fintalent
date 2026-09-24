const loadImage = file => new Promise((resolve, reject) => {
  const url = URL.createObjectURL(file)
  const image = new Image()
  image.onload = () => { URL.revokeObjectURL(url); resolve(image) }
  image.onerror = () => { URL.revokeObjectURL(url); reject(new Error('Не удалось прочитать изображение')) }
  image.src = url
})

export async function resizeImage(file, { maxSize = 800, quality = 0.85 } = {}) {
  const image = await loadImage(file)
  const largestSide = Math.max(image.naturalWidth, image.naturalHeight)
  if (largestSide <= maxSize && file.size <= 5 * 1024 * 1024) return file

  const scale = Math.min(1, maxSize / largestSide)
  const canvas = document.createElement('canvas')
  canvas.width = Math.max(1, Math.round(image.naturalWidth * scale))
  canvas.height = Math.max(1, Math.round(image.naturalHeight * scale))
  const context = canvas.getContext('2d')
  if (!context) throw new Error('Не удалось подготовить изображение')
  context.drawImage(image, 0, 0, canvas.width, canvas.height)

  const blob = await new Promise(resolve => canvas.toBlob(resolve, file.type, quality))
  if (!blob) throw new Error('Не удалось уменьшить изображение')
  return new File([blob], file.name, { type: blob.type || file.type, lastModified: file.lastModified })
}
