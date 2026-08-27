(function(){
  const root=document.querySelector('#hero-rotator');
  if(!root)return;
  const slides=[...root.querySelectorAll('[data-hero-slide]')];
  const dots=[...root.querySelectorAll('.hero-rotator-dots button')];
  const testingText=root.querySelector('#testing-typing');
  const testingWords=['сотрудников','по собственным тестам','с понятным результатом'];
  const profimarketText=root.querySelector('#profimarket-typing');
  const profimarketWords=['готовые решения','полезные шаблоны','опыт экспертов'];
  const clientExchangeText=root.querySelector('#client-exchange-typing');
  const clientExchangeWords=['передача клиентов','новые клиенты','защищённая передача'];
  const reduceMotion=window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  let active=0,timer,typeTimer,word=0,letter=testingWords[0].length,deleting=true,paused=false;

  function typeNext(){
    if(!testingText||active!==1||reduceMotion)return;
    const value=testingWords[word];
    if(deleting){
      letter=Math.max(0,letter-1); testingText.textContent=value.slice(0,letter);
      if(!letter){deleting=false;word=(word+1)%testingWords.length;typeTimer=setTimeout(typeNext,260);return;}
      typeTimer=setTimeout(typeNext,48);return;
    }
    const next=testingWords[word]; letter+=1; testingText.textContent=next.slice(0,letter);
    if(letter>=next.length){deleting=true;typeTimer=setTimeout(typeNext,1800);return;}
    typeTimer=setTimeout(typeNext,76);
  }
  function typeProfimarket(){
    if(!profimarketText||active!==2||reduceMotion)return;
    const value=profimarketWords[word];
    if(deleting){
      letter=Math.max(0,letter-1); profimarketText.textContent=value.slice(0,letter);
      if(!letter){deleting=false;word=(word+1)%profimarketWords.length;typeTimer=setTimeout(typeProfimarket,260);return;}
      typeTimer=setTimeout(typeProfimarket,48);return;
    }
    const next=profimarketWords[word]; letter+=1; profimarketText.textContent=next.slice(0,letter);
    if(letter>=next.length){deleting=true;typeTimer=setTimeout(typeProfimarket,1800);return;}
    typeTimer=setTimeout(typeProfimarket,76);
  }
  function typeClientExchange(){
    if(!clientExchangeText||active!==3||reduceMotion)return;
    const value=clientExchangeWords[word];
    if(deleting){
      letter=Math.max(0,letter-1); clientExchangeText.textContent=value.slice(0,letter);
      if(!letter){deleting=false;word=(word+1)%clientExchangeWords.length;typeTimer=setTimeout(typeClientExchange,260);return;}
      typeTimer=setTimeout(typeClientExchange,48);return;
    }
    const next=clientExchangeWords[word]; letter+=1; clientExchangeText.textContent=next.slice(0,letter);
    if(letter>=next.length){deleting=true;typeTimer=setTimeout(typeClientExchange,1800);return;}
    typeTimer=setTimeout(typeClientExchange,76);
  }
  function show(index){
    if(index===active)return;
    clearTimeout(typeTimer);
    slides[active].classList.add('is-leaving');
    slides[active].classList.remove('is-active');
    slides[active].setAttribute('aria-hidden','true');
    dots[active].classList.remove('active');
    active=index;
    slides[active].classList.remove('is-leaving');
    slides[active].classList.add('is-active');
    slides[active].setAttribute('aria-hidden','false');
    dots[active].classList.add('active');
    if(active===1){word=0;letter=testingWords[0].length;deleting=true;testingText.textContent=testingWords[0];typeTimer=setTimeout(typeNext,1400);}
    if(active===2){word=0;letter=profimarketWords[0].length;deleting=true;profimarketText.textContent=profimarketWords[0];typeTimer=setTimeout(typeProfimarket,1400);}
    if(active===3){
      word=0;letter=clientExchangeWords[0].length;deleting=true;clientExchangeText.textContent=clientExchangeWords[0];typeTimer=setTimeout(typeClientExchange,1400);
    }
  }
  function schedule(){
    clearInterval(timer);
    if(paused||reduceMotion)return;
    timer=setInterval(()=>show((active+1)%slides.length),10000);
  }
  function pause(){paused=true;clearInterval(timer)}
  function resume(){paused=false;schedule()}
  dots.forEach((dot,index)=>dot.addEventListener('click',()=>{show(index);schedule()}));
  root.addEventListener('mouseenter',pause);
  root.addEventListener('mouseleave',resume);
  root.addEventListener('focusin',pause);
  root.addEventListener('focusout',event=>{if(!root.contains(event.relatedTarget))resume()});
  schedule();
})();
