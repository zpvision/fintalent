import EditorJS from '@editorjs/editorjs';
import Header from '@editorjs/header';
import EditorjsList from '@editorjs/list';
import Quote from '@editorjs/quote';
import CodeTool from '@editorjs/code';
import Table from '@editorjs/table';
import Delimiter from '@editorjs/delimiter';
import Marker from '@editorjs/marker';
import Underline from '@editorjs/underline';
import InlineCode from '@editorjs/inline-code';

const icons={
  avoid:'↗',note:'!',example:'◇',conclusion:'✓',warning:'⚠',info:'i',image:'▧',video:'▶'
};

class CalloutTool{
  constructor({data,config}){this.data=data||{};this.config=config||{}}
  static get sanitize(){return{text:{b:true,strong:true,i:true,em:true,u:true,a:{href:true,target:true,rel:true},code:true,mark:true,br:true}}}
  render(){
    const root=document.createElement('div');root.className=`ft-editor-callout ${this.config.kind||''}`;
    root.innerHTML=`<i>${icons[this.config.kind]||'i'}</i><div><b>${this.config.title||'Информационный блок'}</b><div class="ft-editor-callout-text" contenteditable="true" data-placeholder="Введите текст блока…">${this.data.text||''}</div></div>`;
    this.input=root.querySelector('.ft-editor-callout-text');return root;
  }
  save(){return{text:this.input.innerHTML.trim()}}
  static get isReadOnlySupported(){return true}
}

class MediaURLTool{
  constructor({data,config}){this.data=data||{};this.config=config||{}}
  render(){
    const root=document.createElement('div');root.className=`ft-editor-media ${this.config.kind||'image'}`;
    root.innerHTML=`<div class="ft-editor-media-preview"></div><label><b>${this.config.kind==='video'?'Ссылка на видео':'Изображение'}</b><input class="media-url" type="url" placeholder="https://…" value="${escapeAttribute(this.data.url||'')}"></label>${this.config.kind==='image'?'<label><span>Подпись</span><input class="media-caption" value="'+escapeAttribute(this.data.caption||'')+'"></label><label class="media-upload">Загрузить изображение<input type="file" accept="image/png,image/jpeg,image/webp,image/gif"></label>':''}`;
    this.root=root;this.url=root.querySelector('.media-url');this.caption=root.querySelector('.media-caption');this.preview=root.querySelector('.ft-editor-media-preview');
    this.url.addEventListener('input',()=>this.renderPreview());root.querySelector('input[type=file]')?.addEventListener('change',event=>this.upload(event.target.files[0]));this.renderPreview();return root;
  }
  renderPreview(){const url=this.url.value.trim();if(this.config.kind==='image'&&url)this.preview.innerHTML=`<img src="${escapeAttribute(url)}" alt="">`;else if(this.config.kind==='video'&&url)this.preview.innerHTML='<span>▶ Видео будет встроено при просмотре</span>';else this.preview.innerHTML='<span>Добавьте ссылку или загрузите файл</span>'}
  async upload(file){if(!file)return;const form=new FormData();form.append('image',file);this.root.classList.add('loading');try{const response=await fetch('/api/publications/upload',{method:'POST',body:form});const data=await response.json();if(!response.ok)throw new Error(data.error||'Ошибка загрузки');this.url.value=data.url;this.renderPreview()}finally{this.root.classList.remove('loading')}}
  save(){return{url:this.url.value.trim(),caption:this.caption?.value.trim()||''}}
  static get isReadOnlySupported(){return true}
}

function escapeAttribute(value){return String(value).replace(/[&<>"']/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]))}
function listItems(items){return(items||[]).map(item=>typeof item==='string'?{content:item,meta:{},items:[]}:{content:item.content||item.text||'',meta:item.meta||{},items:listItems(item.items)})}
function flattenList(items,result=[]){for(const item of items||[]){result.push(item.content||item.text||'');flattenList(item.items,result)}return result}

function fromLegacy(blocks){
  return{blocks:(blocks||[]).map(block=>{
    if(block.type==='h2'||block.type==='h3')return{type:'header',data:{text:block.text||'',level:block.type==='h2'?2:3}};
    if(block.type==='bullets'||block.type==='numbered'||block.type==='checklist')return{type:'list',data:{style:block.type==='numbered'?'ordered':block.type==='checklist'?'checklist':'unordered',meta:{},items:listItems(block.items)}};
    if(block.type==='quote')return{type:'quote',data:{text:block.text||'',caption:'',alignment:'left'}};
    if(block.type==='code')return{type:'code',data:{code:block.text||''}};
    if(block.type==='table')return{type:'table',data:{withHeadings:true,content:block.rows||[]}};
    if(block.type==='divider')return{type:'delimiter',data:{}};
    if(['avoid','note','example','conclusion','warning','info'].includes(block.type))return{type:block.type,data:{text:block.text||''}};
    if(block.type==='image')return{type:'image',data:{url:block.url||'',caption:block.caption||''}};
    if(block.type==='video')return{type:'video',data:{url:block.url||''}};
    return{type:'paragraph',data:{text:block.text||''}};
  })};
}

function toLegacy(output){
  return(output?.blocks||[]).map(block=>{
    const data=block.data||{};
    if(block.type==='header')return{type:data.level===3?'h3':'h2',text:data.text||''};
    if(block.type==='list')return{type:data.style==='ordered'?'numbered':data.style==='checklist'?'checklist':'bullets',items:flattenList(data.items)};
    if(block.type==='quote')return{type:'quote',text:data.text||''};
    if(block.type==='code')return{type:'code',text:data.code||''};
    if(block.type==='table')return{type:'table',rows:data.content||[]};
    if(block.type==='delimiter')return{type:'divider'};
    if(['avoid','note','example','conclusion','warning','info'].includes(block.type))return{type:block.type,text:data.text||''};
    if(block.type==='image')return{type:'image',url:data.url||'',caption:data.caption||''};
    if(block.type==='video')return{type:'video',url:data.url||''};
    return{type:'paragraph',text:data.text||''};
  }).filter(block=>block.type==='divider'||block.text||block.url||block.items?.length||block.rows?.length);
}

async function mount({holder,data,onChange}){
  const callout=(kind,title)=>({class:CalloutTool,inlineToolbar:true,config:{kind,title},toolbox:{title,icon:`<span>${icons[kind]}</span>`}});
  const editor=new EditorJS({
    holder,
    data:fromLegacy(data),
    placeholder:'Начните писать экспертный материал…',
    minHeight:320,
    inlineToolbar:['bold','italic','underline','marker','inlineCode','link'],
    tools:{
      header:{class:Header,inlineToolbar:true,config:{levels:[2,3],defaultLevel:2}},
      list:{class:EditorjsList,inlineToolbar:true,config:{defaultStyle:'unordered',maxLevel:2}},
      quote:{class:Quote,inlineToolbar:true,config:{quotePlaceholder:'Текст цитаты',captionPlaceholder:'Источник'}},
      code:CodeTool,table:{class:Table,inlineToolbar:true,config:{rows:2,cols:3}},delimiter:Delimiter,
      marker:Marker,underline:Underline,inlineCode:InlineCode,
      avoid:callout('avoid','Как избежать'),note:callout('note','Обратите внимание'),example:callout('example','Практический пример'),conclusion:callout('conclusion','Вывод'),warning:callout('warning','Предупреждение'),info:callout('info','Информация'),
      image:{class:MediaURLTool,config:{kind:'image'},toolbox:{title:'Изображение',icon:'<span>▧</span>'}},video:{class:MediaURLTool,config:{kind:'video'},toolbox:{title:'Видео',icon:'<span>▶</span>'}},
    },
    i18n:{messages:{ui:{blockTunes:{toggler:{'Click to tune':'Настройки блока','or drag to move':'или перетащите'}}},toolNames:{Text:'Абзац',Heading:'Заголовок','Unordered List':'Список','Ordered List':'Нумерация',Checklist:'Чек-лист',Quote:'Цитата',Code:'Код',Table:'Таблица',Delimiter:'Разделитель'},blockTunes:{delete:{Delete:'Удалить'},moveUp:{'Move up':'Выше'},moveDown:{'Move down':'Ниже'}}}},
    onChange:()=>onChange?.(),
  });
  await editor.isReady;
  return{editor,save:async()=>toLegacy(await editor.save())};
}

window.FinTalentEditorJS={mount,fromLegacy,toLegacy};
