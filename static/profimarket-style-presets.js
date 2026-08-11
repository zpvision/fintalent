(()=>{
  const presets=[
    {key:'metrics-default',name:'Ключевые показатели · текущий',section:'#ffffff',text:'#11183c',card:'#ffffff',iconBg:'transparent',icon:'#6548ec',heading:'#11183c',border:'#eceff7'},
    {key:'access-default',name:'Фиолетовый контур',section:'#ffffff',text:'#11183c',card:'#ffffff',iconBg:'#f1efff',icon:'#5639e5',heading:'#11183c',border:'#826dff'},
    {key:'amber',name:'Тёплый янтарь',section:'#fff9e9',text:'#8a5910',card:'#fff8e8',iconBg:'#fff0cc',icon:'#bc7510',heading:'#b87311',border:'#eedcb5'},
    {key:'violet',name:'Современный фиолетовый',section:'#f5f1ff',text:'#4931a8',card:'#eee8ff',iconBg:'#ddd2ff',icon:'#6847dc',heading:'#5e42bc',border:'#d9ccf5'},
    {key:'ocean',name:'Глубокий океан',section:'#edf7ff',text:'#174d7a',card:'#e3f2ff',iconBg:'#cce8ff',icon:'#1976b9',heading:'#17689d',border:'#c8e3f4'},
    {key:'mint',name:'Свежая мята',section:'#edfbf6',text:'#176a51',card:'#e1f7ee',iconBg:'#c8efdf',icon:'#15966e',heading:'#168361',border:'#c5e9dc'},
    {key:'rose',name:'Пудровая роза',section:'#fff2f6',text:'#8a3150',card:'#ffe7ef',iconBg:'#ffd2e0',icon:'#c64e78',heading:'#ae4167',border:'#f0cad7'},
    {key:'graphite',name:'Мягкий графит',section:'#f2f4f7',text:'#30394a',card:'#e7ebf0',iconBg:'#d7dde5',icon:'#526176',heading:'#465267',border:'#d5dbe3'},
    {key:'sky',name:'Индиго и сирень',section:'linear-gradient(125deg,#eef0ff,#f5ecff)',text:'#393579',card:'#e8e7ff',iconBg:'#d8d4ff',icon:'#5952ba',heading:'#4d469c',border:'#cbc8ee'},
    {key:'terracotta',name:'Терракота',section:'#fff3ed',text:'#88462f',card:'#ffe8dd',iconBg:'#ffd5c5',icon:'#c56643',heading:'#a95639',border:'#efcdbf'},
    {key:'lavender',name:'Лавандовый градиент',section:'linear-gradient(120deg,#f7f2ff,#eef3ff)',text:'#513d86',card:'#f1ebff',iconBg:'#ded3fa',icon:'#7659b8',heading:'#674da4',border:'#dad0ed'},
    {key:'aurora',name:'Персиковый закат',section:'linear-gradient(125deg,#fff1e8,#ffeef5)',text:'#843f4f',card:'#ffe5df',iconBg:'#ffd2cf',icon:'#c95768',heading:'#aa4859',border:'#efc4c9'},
    {key:'sand',name:'Спокойный песок',section:'#faf6ed',text:'#6e5830',card:'#f4ecdc',iconBg:'#eadcbe',icon:'#9b793b',heading:'#86672f',border:'#e4d9c3'}
  ];
  window.ProfiMarketStylePresets={presets,byKey:Object.fromEntries(presets.map(item=>[item.key,item])),defaults:{metrics:'metrics-default',access:'access-default',bonuses:'amber'}};
})();
