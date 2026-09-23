'use strict';

(() => {
  const section = document.querySelector('#recommendations');
  if (!section) return;
  const { request, node } = window.TaskLab;
  let revision = 0;

  async function load() {
    const version = ++revision;
    let teamID;
    try { teamID = sessionStorage.getItem('tasklab-selected-team'); }
    catch {
      section.hidden = false;
      section.replaceChildren(node('p', 'Не удалось прочитать выбранную команду. Разрешите хранение данных в браузере.', 'message'));
      return;
    }
    section.hidden = false;
    if (!teamID) {
      const link = node('a', 'Выбрать команду →', 'text-link');
      link.href = '/static/task-builder/teams.html';
      section.replaceChildren(node('h2', 'Задачи по интересам команды'), node('p', 'Выберите команду, чтобы получить рекомендации.', 'hint'), link);
      return;
    }
    section.replaceChildren(node('p', 'Подбираем задачи по интересам команды…', 'hint'));
    try {
      const items = await request(`/api/teams/${encodeURIComponent(teamID)}/recommendations`);
      if (version !== revision) return;
      const list = node('div', undefined, 'catalog');
      for (const item of items) {
        const card = node('article', undefined, `card level-${item.level}`);
        const link = node('a', item.title);
        link.href = `/static/task-builder/task.html?id=${encodeURIComponent(item.id)}`;
        const title = node('h3'); title.append(link);
        card.append(title,
          node('p', [item.company, item.category_label].filter(Boolean).join(' · '), 'meta'),
          node('p', `${item.score} / 100 · ${item.level_label}`, 'badge'),
          node('p', item.reason, 'need'));
        list.append(card);
      }
      const catalog = node('a', 'Посмотреть все задачи →', 'text-link'); catalog.href = '/';
      section.replaceChildren(node('h2', 'Рекомендовано для команды'),
        items.length ? list : node('p', 'Подходящих рекомендаций пока нет. Вы можете выбрать любую задачу в каталоге.', 'hint'));
      const footer = node('p'); footer.append(catalog); section.append(footer);
    } catch (error) {
      if (version !== revision) return;
      const message = node('p', error.message, 'message'); message.setAttribute('role', 'alert');
      const retry = node('button', 'Повторить', 'secondary'); retry.type = 'button'; retry.addEventListener('click', load);
      section.replaceChildren(node('h2', 'Рекомендации недоступны'), message, retry);
    }
  }
  document.addEventListener('team:selected', load);
  load();
})();
