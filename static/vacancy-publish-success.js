function showPublishedModal(){
  document.querySelector('.publish-success-modal')?.remove()
  const modal=document.createElement('div')
  modal.className='publish-success-modal'
  modal.innerHTML=`<div class="publish-confetti" aria-hidden="true">${Array.from({length:18},(_,index)=>`<i style="--i:${index}"></i>`).join('')}</div>
    <section role="dialog" aria-modal="true" aria-labelledby="publish-success-title">
      <button type="button" class="publish-success-close" data-publish-close aria-label="Закрыть">×</button>
      <div class="publish-success-mark" aria-hidden="true"><span>✓</span></div>
      <small>ВАКАНСИЯ ОПУБЛИКОВАНА</small>
      <h2 id="publish-success-title">Всё получилось!</h2>
      <p>Вы проделали большую работу — вакансия уже опубликована и готова встретить подходящих кандидатов.</p>
      <div class="publish-success-wish"><i>✦</i><span><b>Пусть нужный специалист найдётся совсем скоро</b><small>Желаем сильной команды и отличного сотрудничества!</small></span></div>
      <div class="publish-success-actions"><a href="/profile?section=vacancies">Перейти к моим вакансиям <span>→</span></a><button type="button" data-publish-close>Остаться здесь</button></div>
    </section>`
  document.body.append(modal)
  const close=()=>{modal.classList.add('closing');setTimeout(()=>modal.remove(),180)}
  modal.querySelectorAll('[data-publish-close]').forEach(button=>button.onclick=close)
  modal.onclick=event=>{if(event.target===modal)close()}
  const onKey=event=>{if(event.key==='Escape'){document.removeEventListener('keydown',onKey);close()}}
  document.addEventListener('keydown',onKey)
  modal.querySelector('.publish-success-close').focus()
}
