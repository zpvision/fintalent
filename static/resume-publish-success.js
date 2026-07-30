(function(){
 window.showResumePublishedModal=async function(resumeID){
  if(!resumeID){
   try{
    const response=await fetch('/api/v1/resumes/status',{cache:'no-store'});
    if(response.ok)resumeID=(await response.json()).id;
   }catch{}
  }
  document.querySelector('.publish-success-modal')?.remove();
  const modal=document.createElement('div');
  modal.className='publish-success-modal';
  const viewURL=resumeID?`/resume/view/${encodeURIComponent(resumeID)}`:'/profile';
  modal.innerHTML=`<div class="publish-confetti" aria-hidden="true">${Array.from({length:18},(_,index)=>`<i style="--i:${index}"></i>`).join('')}</div><section role="dialog" aria-modal="true" aria-labelledby="resume-publish-success-title"><button type="button" class="publish-success-close" data-resume-publish-close aria-label="Закрыть">×</button><div class="publish-success-mark" aria-hidden="true"><span>✓</span></div><small>РЕЗЮМЕ ОПУБЛИКОВАНО</small><h2 id="resume-publish-success-title">Отличное начало!</h2><p>Вы проделали большую работу — резюме уже доступно работодателям и готово привести вас к новым возможностям.</p><div class="publish-success-wish"><i>✦</i><span><b>Мы верим в ваш талант</b><small>Желаем найти работу, на которой вас будут ценить.</small></span></div><div class="publish-success-actions"><a href="${viewURL}">Перейти к просмотру резюме <span>→</span></a><button type="button" data-resume-publish-close>Остаться здесь</button></div></section>`;
  document.body.append(modal);
  const close=()=>{modal.classList.add('closing');setTimeout(()=>modal.remove(),180)};
  modal.querySelectorAll('[data-resume-publish-close]').forEach(button=>button.onclick=close);
  modal.onclick=event=>{if(event.target===modal)close()};
  const onKey=event=>{if(event.key==='Escape'){document.removeEventListener('keydown',onKey);close()}};
  document.addEventListener('keydown',onKey);
  modal.querySelector('.publish-success-close').focus();
 };
})();
