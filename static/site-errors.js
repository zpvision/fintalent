(function(){
  const style=document.createElement('link');style.rel='stylesheet';style.href='/static/site-errors.css?v=1';document.head.append(style);
  let lastMessage='',lastShown=0;
  function messageOf(value){
    if(!value)return 'Неизвестная ошибка';
    if(typeof value==='string')return value;
    if(value.error)return String(value.error);
    if(value.message)return String(value.message);
    return 'Не удалось выполнить операцию';
  }
  window.showFinTalentError=function(value){
    const message=messageOf(value).trim()||'Не удалось выполнить операцию',now=Date.now();
    if(message===lastMessage&&now-lastShown<2500)return;
    lastMessage=message;lastShown=now;
    let container=document.querySelector('#ft-error-toasts');
    if(!container){container=document.createElement('div');container.id='ft-error-toasts';container.setAttribute('aria-live','assertive');document.body.append(container)}
    const toast=document.createElement('aside');toast.className='ft-error-toast';
    const icon=document.createElement('i');icon.textContent='!';
    const content=document.createElement('div'),title=document.createElement('b'),text=document.createElement('p'),support=document.createElement('span');
    title.textContent='Что-то пошло не так';text.textContent=message;support.textContent='Сообщите в FinTalent';content.append(title,text,support);
    const close=document.createElement('button');close.type='button';close.setAttribute('aria-label','Закрыть');close.textContent='×';close.onclick=()=>removeToast(toast);
    toast.append(icon,content,close);container.append(toast);requestAnimationFrame(()=>toast.classList.add('show'));setTimeout(()=>removeToast(toast),9000);
  };
  function removeToast(toast){if(!toast?.isConnected)return;toast.classList.remove('show');setTimeout(()=>toast.remove(),220)}
  const nativeFetch=window.fetch.bind(window);
  window.fetch=async function(input,options){
    const url=typeof input==='string'?input:input?.url||'';
    try{
      const response=await nativeFetch(input,options);
      const expectedUnauthorized=response.status===401&&(url.includes('/api/me')||url.includes('/api/admin/session'));
      if(!response.ok&&!expectedUnauthorized){
        let payload;try{payload=await response.clone().json()}catch{try{payload=await response.clone().text()}catch{}}
        window.showFinTalentError(messageOf(payload)||`Ошибка сервера ${response.status}`);
      }
      return response;
    }catch(error){window.showFinTalentError(error?.message||'Нет соединения с сервером');throw error}
  };
  window.addEventListener('unhandledrejection',event=>{const message=messageOf(event.reason);if(message&&message!=='Ошибка')window.showFinTalentError(message)});
  window.addEventListener('error',event=>{if(event.error?.message)window.showFinTalentError(event.error.message)});
})();
