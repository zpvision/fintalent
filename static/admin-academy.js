(() => {
 if(!document.querySelector('script[data-react-admin]'))return;
 const button=document.querySelector('#courses-admin-nav'),workspace=document.querySelector('.workspace');if(!button||!workspace)return;
 const original=button.onclick;window.openLegacyAcademySettings=()=>{original();history.replaceState({},'','/admin/courses');};const label=document.createElement('span');label.textContent='Академия';button.replaceChildren(document.createTextNode('▤ '),label);
 const section=document.createElement('section');section.id='admin-academy';section.className='hidden';workspace.append(section);
 button.onclick=()=>{document.querySelectorAll('.workspace>section,.dictionary-list,.dictionary-editor').forEach(el=>el.classList.add('hidden'));section.classList.remove('hidden');document.querySelectorAll('.sidebar nav button').forEach(el=>el.classList.toggle('active',el===button));document.querySelectorAll('.workspace>header .primary').forEach(el=>el.classList.add('hidden'));document.querySelector('.workspace>header h1').textContent='Академия';document.querySelector('.workspace>header p').textContent='Курсы, обучение и техническая поддержка FinTalent';window.dispatchEvent(new Event('academy-admin-open'));};
 document.querySelector('.sidebar nav').addEventListener('click',e=>{if(e.target.closest('button')!==button){section.classList.add('hidden');window.dispatchEvent(new Event('academy-admin-close'))}});
 window.dispatchEvent(new Event('academy-admin-ready'));
})();
