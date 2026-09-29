(() => {
  const root = document.querySelector('#result');
  const id = new URLSearchParams(location.search).get('invitation') || '';
  const next = encodeURIComponent(location.pathname + location.search);
  function message(title, body, links = false) {
    root.replaceChildren();
    const h = document.createElement('h1'); h.textContent = title; root.append(h);
    const p = document.createElement('p'); p.textContent = body; root.append(p);
    if (links) {
      const login = document.createElement('a'); login.href = '/login?next=' + next; login.textContent = 'Войти'; root.append(login);
      root.append(document.createTextNode(' · '));
      const register = document.createElement('a'); register.href = '/register?next=' + next; register.textContent = 'Зарегистрироваться'; root.append(register);
    }
  }
  if (!/^\d+$/.test(id)) { message('Ссылка недоступна', 'Проверьте адрес ссылки.'); return; }
  fetch('/api/employee-testing/result-review/' + encodeURIComponent(id), { credentials: 'include', cache: 'no-store' }).then(async response => {
    if (response.status === 401) { message('Войдите, чтобы посмотреть результат', 'Разбор доступен только в аккаунте с адресом, на который отправлено письмо.', true); return; }
    if (!response.ok) { message('Результат недоступен', 'Войдите под адресом, на который пришло письмо, или обратитесь к организатору.', true); return; }
    const result = await response.json(); root.replaceChildren();
    const heading = document.createElement('h1'); heading.textContent = result.test_title || 'Разбор результатов'; root.append(heading);
    const sub = document.createElement('p'); sub.textContent = 'Ваш результат: ' + Math.round(result.percent || 0) + '%'; root.append(sub);
    const title = document.createElement('h2'); title.textContent = 'Разбор результатов · Проверьте свои ответы'; root.append(title);
    const answers = new Map(); (result.answers || []).forEach(answer => { const group = answers.get(answer.question_id) || { ...answer, ids: [] }; if (answer.selected_answer_id) group.ids.push(answer.selected_answer_id); answers.set(answer.question_id, group); });
    (result.questions || []).forEach((question, index) => {
      const article = document.createElement('article'); article.className = 'review-question';
      const h = document.createElement('h3'); h.textContent = 'Вопрос ' + (index + 1) + '. ' + question.question; article.append(h);
      const answer = answers.get(question.id);
      const state = document.createElement('p'); state.textContent = answer?.is_correct ? '✓ Верно' : '× Есть ошибка'; article.append(state);
      if (question.question_type === 'text') { const p = document.createElement('p'); p.textContent = 'Ваш ответ: ' + (answer?.text_answer || 'Ответ не дан'); article.append(p); }
      else { const list = document.createElement('ul'); (question.answers || []).forEach(option => { const item = document.createElement('li'); item.textContent = (option.is_correct ? '✓ ' : answer?.ids.includes(option.id) ? '× ' : '  ') + option.answer; list.append(item); }); article.append(list); }
      if (question.explanation) { const p = document.createElement('p'); p.textContent = question.explanation; article.append(p); }
      root.append(article);
    });
  }).catch(() => message('Не удалось загрузить результат', 'Попробуйте обновить страницу.'));
})();
