import { useEffect, useState } from 'react'

export default function useTypingWords(words, options = {}) {
  const {
    enabled = true,
    initialDelay = 1400,
    deleteDelay = 48,
    typeDelay = 76,
    wordDelay = 1800,
    emptyDelay = 260,
  } = options
  const [text, setText] = useState(words[0] || '')
  const wordsKey = words.join('\u0000')

  useEffect(() => {
    const values = wordsKey.split('\u0000')
    setText(values[0] || '')
    if (!enabled || window.matchMedia('(prefers-reduced-motion: reduce)').matches || !values.length) return undefined

    let wordIndex = 0
    let letterIndex = values[0].length
    let deleting = true
    let timer

    function next() {
      const value = values[wordIndex]
      if (deleting) {
        letterIndex = Math.max(0, letterIndex - 1)
        setText(value.slice(0, letterIndex))
        if (!letterIndex) {
          deleting = false
          wordIndex = (wordIndex + 1) % values.length
          timer = window.setTimeout(next, emptyDelay)
          return
        }
        timer = window.setTimeout(next, deleteDelay)
        return
      }

      const nextValue = values[wordIndex]
      letterIndex += 1
      setText(nextValue.slice(0, letterIndex))
      if (letterIndex >= nextValue.length) {
        deleting = true
        timer = window.setTimeout(next, wordDelay)
        return
      }
      timer = window.setTimeout(next, typeDelay)
    }

    timer = window.setTimeout(next, initialDelay)
    return () => window.clearTimeout(timer)
  }, [wordsKey, enabled, initialDelay, deleteDelay, typeDelay, wordDelay, emptyDelay])

  return text
}
