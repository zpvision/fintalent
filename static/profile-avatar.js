(()=>{
  document.head.insertAdjacentHTML('beforeend','<link rel="stylesheet" href="/static/profile-avatar.css?v=1">');
  const main=document.querySelector('.dashboard-main');
  if(!main)return;
  main.insertAdjacentHTML('afterbegin','<section class="profile-photo-card"><div class="profile-photo-preview" id="profile-photo-preview">Я</div><div><h2>Фотография профиля</h2><p>Она будет показана в вашем профиле, публикациях и решениях ПрофиМаркета.</p><label class="profile-photo-button">Выбрать фотографию<input id="profile-photo-input" type="file" accept="image/jpeg,image/png,image/webp"></label><small id="profile-photo-status">JPG, PNG или WebP, до 20 МБ · большие фото уменьшим автоматически</small></div></section>');
  const paint=(element,url,initial)=>{if(element)element.innerHTML=url?`<img src="${url}" alt="Фотография профиля">`:initial};
  const resize=async file=>{
    const url=URL.createObjectURL(file),image=new Image();
    try{await new Promise((resolve,reject)=>{image.onload=resolve;image.onerror=()=>reject(Error('Не удалось прочитать изображение'));image.src=url})}finally{URL.revokeObjectURL(url)}
    const largest=Math.max(image.naturalWidth,image.naturalHeight);if(largest<=800&&file.size<=5*1024*1024)return file;
    const scale=Math.min(1,800/largest),canvas=document.createElement('canvas');canvas.width=Math.max(1,Math.round(image.naturalWidth*scale));canvas.height=Math.max(1,Math.round(image.naturalHeight*scale));
    const context=canvas.getContext('2d');if(!context)throw Error('Не удалось подготовить изображение');context.drawImage(image,0,0,canvas.width,canvas.height);
    const blob=await new Promise(resolve=>canvas.toBlob(resolve,file.type,.85));if(!blob)throw Error('Не удалось уменьшить изображение');return new File([blob],file.name,{type:blob.type||file.type,lastModified:file.lastModified});
  };
  fetch('/api/me').then(async response=>{if(!response.ok)return;const user=await response.json(),initial=(user.full_name||'П').trim().charAt(0).toUpperCase();paint(document.querySelector('#header-avatar'),user.avatar,initial);paint(document.querySelector('#sidebar-avatar'),user.avatar,initial);paint(document.querySelector('#profile-photo-preview'),user.avatar,initial)});
  const input=document.querySelector('#profile-photo-input'),status=document.querySelector('#profile-photo-status');
  input.onchange=async()=>{const file=input.files[0];if(!file)return;if(file.size>20*1024*1024){status.textContent='Исходный файл больше 20 МБ';return}status.textContent='Подготавливаем фотографию…';let upload=file;try{try{upload=await resize(file)}catch{upload=file}if(upload.size>5*1024*1024)throw Error('Не удалось уменьшить фотографию до 5 МБ');status.textContent='Загружаем…';const body=new FormData();body.append('avatar',upload);const response=await fetch('/api/profile/avatar',{method:'POST',body}),data=await response.json().catch(()=>({}));if(!response.ok)throw Error(data.error||'Не удалось загрузить фотографию');const initial='П';paint(document.querySelector('#header-avatar'),data.avatar,initial);paint(document.querySelector('#sidebar-avatar'),data.avatar,initial);paint(document.querySelector('#profile-photo-preview'),data.avatar,initial);status.textContent='Фотография сохранена'}catch(error){status.textContent=error.message}finally{input.value=''}};
})();
