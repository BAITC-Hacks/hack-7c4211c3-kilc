'use strict';

(() => {
  const section = document.querySelector('#recommendations');
  if (!section || location.pathname !== '/') return;

  let teamID;
  try { teamID = sessionStorage.getItem('tasklab-selected-team'); }
  catch { return; }
  if (!teamID) return;

  fetch(`/api/teams/${encodeURIComponent(teamID)}/recommendations`)
    .then(response => {
      if (!response.ok) throw new Error('Не удалось загрузить рекомендации');
      return response.json();
    })
    .then(items => {
      if (!Array.isArray(items) || items.length === 0) return;
      const heading = document.createElement('h2');
      heading.textContent = 'Рекомендовано для команды';
      const list = document.createElement('div');
      list.className = 'catalog';
      for (const item of items) {
        const card = document.createElement('article');
        card.className = `card level-${item.level}`;
        const link = document.createElement('a');
        link.href = `/static/task-builder/task.html?id=${encodeURIComponent(item.id)}`;
        link.textContent = item.title;
        const title = document.createElement('h3');
        title.append(link);
        const meta = document.createElement('p');
        meta.className = 'meta';
        meta.textContent = `${item.company ? `${item.company} · ` : ''}${item.category_label} · ${item.score} / 100`;
        const reason = document.createElement('p');
        reason.className = 'need';
        reason.textContent = item.reason;
        card.append(title, meta, reason);
        list.append(card);
      }
      section.replaceChildren(heading, list);
      section.hidden = false;
    })
    .catch(error => console.error(error));
})();
