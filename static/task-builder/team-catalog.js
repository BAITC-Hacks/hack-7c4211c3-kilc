'use strict';
(() => {
 const {request,node}=TaskLab;
 const $=s=>document.querySelector(s);
 let teams=[],categories=[];
 const label=code=>categories.find(c=>c.code===code)?.label||code;
 function render(){
  const query=$('#team-search').value.trim().toLocaleLowerCase('ru');
  const specialty=$('#specialty').value;
  const items=teams.filter(t=>(!specialty||(t.interests||[]).includes(specialty))&&[t.name,t.skills,t.tech,...(t.interests||[]).map(label)].join(' ').toLocaleLowerCase('ru').includes(query));
  $('#teams-list').replaceChildren();$('#teams-count').textContent=`Команд: ${items.length}`;
  if(!items.length)$('#teams-list').append(node('p','Команды не найдены. Попробуйте другой запрос.','empty'));
  for(const team of items){
   const card=node('article',undefined,'section team-card');
   card.append(node('div',team.name.slice(0,2).toUpperCase(),'team-icon'),node('h2',team.name));
   const tags=node('div',undefined,'tags');for(const code of team.interests||[])tags.append(node('span',label(code),'tag'));
   card.append(node('p',`Специализации: ${(team.interests||[]).length?'':'Не указаны'}`),tags,node('p',`Навыки: ${team.skills||'Не указаны'}`),node('p',`Технологии: ${team.tech||'Не указаны'}`),node('p',`Баллы за выполненные этапы: ${team.points}`,'team-meta'));
   $('#teams-list').append(card);
  }
 }
 async function load(){
  $('#reload-teams').disabled=true;$('#teams-error').textContent='';
  try {
   await TaskLab.ready;
   [teams,categories]=await Promise.all([request('/api/teams'),request('/api/categories')]);
   const value=$('#specialty').value;$('#specialty').replaceChildren(new Option('Все специализации',''));
   for(const category of categories)$('#specialty').append(new Option(category.label,category.code));
   $('#specialty').value=value;render();
  }catch(e){$('#teams-error').textContent=e.message;}finally{$('#reload-teams').disabled=false;}
 }
 $('#team-search').addEventListener('input',render);$('#specialty').addEventListener('change',render);$('#reload-teams').addEventListener('click',load);load();
})();
