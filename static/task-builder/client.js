'use strict';
window.TaskLab = {
  async request(path, options = {}) {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 12000);
    try {
      const response = await fetch(path, { ...options, signal: controller.signal,
        headers: { 'Content-Type': 'application/json', ...options.headers } });
      const text = await response.text();
      let data;
      try { data = JSON.parse(text); } catch { throw new Error('Сервер вернул неверный ответ. Откройте страницы через Go-сервер, а не как локальный HTML.'); }
      if (!response.ok) throw new Error(data.error || `Ошибка сервера: ${response.status}`);
      return data;
    } catch (error) {
      if (error.name === 'AbortError') throw new Error('Сервер не ответил вовремя. Проверьте соединение и повторите действие.');
      if (error instanceof TypeError) throw new Error('Нет соединения с API. Запустите go run . и откройте http://localhost:8080.');
      throw error;
    } finally { clearTimeout(timeout); }
  },
  node(tag, text, className) {
    const node = document.createElement(tag);
    if (text !== undefined) node.textContent = text;
    if (className) node.className = className;
    return node;
  }
};

TaskLab.ready = TaskLab.request('/api/session').then(session => {
  TaskLab.session = session;
  try {
    sessionStorage.setItem('tasklab-role', session.role);
    if (session.team_id) sessionStorage.setItem('tasklab-selected-team', String(session.team_id));
    else sessionStorage.removeItem('tasklab-selected-team');
  } catch { /* Server session remains authoritative when storage is unavailable. */ }
  return session;
});
// Consumers display their own contextual error; avoid an unhandled rejection.
TaskLab.ready.catch(() => {});
