(function () {
  const element = document.querySelector('#typing-role');
  if (!element) return;

  const words = ['бухгалтеров', 'финансистов', 'руководителей', 'директоров'];
  const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  if (reducedMotion) {
    element.textContent = words[0];
    return;
  }

  let wordIndex = 0;
  let letterIndex = words[0].length;
  let deleting = true;

  function next() {
    const word = words[wordIndex];

    if (deleting) {
      letterIndex -= 1;
      element.textContent = word.slice(0, Math.max(0, letterIndex));
      if (letterIndex <= 0) {
        deleting = false;
        wordIndex = (wordIndex + 1) % words.length;
        window.setTimeout(next, 280);
        return;
      }
      window.setTimeout(next, 52 + Math.random() * 24);
      return;
    }

    const nextWord = words[wordIndex];
    letterIndex += 1;
    element.textContent = nextWord.slice(0, letterIndex);
    if (letterIndex >= nextWord.length) {
      deleting = true;
      window.setTimeout(next, 1750);
      return;
    }
    window.setTimeout(next, 86 + Math.random() * 42);
  }

  window.setTimeout(next, 1500);
})();
