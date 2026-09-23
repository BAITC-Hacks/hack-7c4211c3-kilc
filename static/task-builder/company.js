'use strict';
(() => {
 const {request}=TaskLab;
 const form=document.querySelector('#company-form');
 const error=document.querySelector('#company-error');
 TaskLab.ready.then(async()=>{
  const companies=await request('/api/business/companies');
  for(const company of companies) document.querySelector('#companies').append(new Option(company,company));
 }).catch(e=>{error.textContent=e.message;});
 form.addEventListener('submit',async event=>{
  event.preventDefault();const button=form.querySelector('button');if(button.disabled)return;
  const company=form.elements.company.value.trim();if(!company){error.textContent='Укажите название компании.';return;}
  if(!confirm(`Закрепить компанию «${company}» за вашей сессией? Сменить компанию после этого нельзя.`))return;
  button.disabled=true;error.textContent='';
  try { await request('/api/session/company',{method:'POST',body:JSON.stringify({company})});location.href='business.html'; }
  catch(e){error.textContent=e.message;button.disabled=false;}
 });
})();
