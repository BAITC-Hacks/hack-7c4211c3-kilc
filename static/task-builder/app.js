'use strict';

// Поля можно переиспользовать при подключении API или AI-помощника.
const fields = [
  { id: 'title', group: 'basics', label: 'Как называется ваша задача?', placeholder: 'Например: автоматизация обработки заявок', hint: 'Короткое и понятное название.', type: 'text', max: 120, required: true },
  { id: 'topic', group: 'basics', label: 'К какой теме относится задача?', type: 'select', options: ['Автоматизация', 'Аналитика данных', 'Маркетинг', 'Образование', 'Другое'], hint: 'Используется для фильтрации в каталоге.' },
  { id: 'context', group: 'basics', label: 'Как процесс устроен сейчас?', placeholder: 'Заявки приходят по почте. Менеджер вручную переносит их в таблицу…', hint: 'Опишите текущую ситуацию.', max: 2000 },
  { id: 'need', group: 'basics', label: 'Какую проблему нужно решить?', placeholder: 'Хотим уменьшить ручной ввод и количество потерянных заявок…', hint: 'Что мешает бизнесу и что необходимо изменить?', max: 2000 },
  { id: 'users', group: 'basics', label: 'Кто будет пользоваться решением?', placeholder: 'Три менеджера отдела продаж и руководитель…', hint: 'Назовите пользователей и их задачи.', max: 1000 },
  { id: 'data', group: 'details', label: 'Какие данные и материалы доступны?', placeholder: 'Обезличенная таблица заявок за месяц, примеры писем…', hint: 'Укажите формат, примеры или источники. Если данных нет — так и напишите.', max: 2000 },
  { id: 'result', group: 'details', label: 'Что вы хотите получить в результате?', placeholder: 'Веб-прототип, который собирает заявки в единый список…', hint: 'Опишите конкретный результат работы команды.', max: 2000 },
  { id: 'success', group: 'details', label: 'Как вы поймёте, что задача решена?', placeholder: 'Обработка тестовой заявки занимает не более 1 минуты…', hint: 'Добавьте измеримые критерии приёмки.', max: 2000 },
  { id: 'constraints', group: 'details', label: 'Какие есть ограничения?', placeholder: 'Прототип за 2 недели; только обезличенные данные…', hint: 'Сроки, технологии, доступы, бюджет или другие границы.', max: 2000 },
  { id: 'contact', group: 'contact', label: 'Как с вами связаться?', placeholder: 'Имя и рабочая почта или Telegram', hint: 'Контакт представителя бизнеса для команды.', type: 'text', max: 250 },
  { id: 'interaction', group: 'contact', label: 'Как будет организована обратная связь?', placeholder: 'Созвон по вторникам, ответы на вопросы в течение рабочего дня…', hint: 'Укажите формат консультаций и порядок обратной связи.', max: 1000 }
];

const criteria = [
  { label: 'Контекст и потребность', weight: 20, fields: ['context', 'need'] },
  { label: 'Данные и материалы', weight: 20, fields: ['data'] },
  { label: 'Ожидаемый результат', weight: 15, fields: ['result'] },
  { label: 'Критерии успеха', weight: 15, fields: ['success'] },
  { label: 'Ограничения', weight: 10, fields: ['constraints'] },
  { label: 'Пользователи', weight: 10, fields: ['users'] },
  { label: 'Связь с бизнесом', weight: 10, fields: ['contact', 'interaction'] }
];
const STORAGE_KEY = 'tasklab-business-draft-v1';
const form = document.querySelector('#task-form');
const confirmation = document.querySelector('#confirmed');
let currentCard = null;

for (const field of fields) {
  const wrapper = document.createElement('div');
  wrapper.className = 'field';
  const label = document.createElement('label');
  label.htmlFor = field.id;
  label.textContent = field.label + (field.required ? ' *' : '');
  const control = document.createElement(field.type === 'select' ? 'select' : field.type === 'text' ? 'input' : 'textarea');
  control.id = field.id;
  control.name = field.id;
  control.required = Boolean(field.required);
  control.setAttribute('aria-describedby', `${field.id}-hint`);
  if (field.type === 'select') {
    for (const value of ['', ...field.options]) {
      const option = document.createElement('option');
      option.value = value;
      option.textContent = value || 'Выберите тему';
      control.append(option);
    }
  } else {
    control.placeholder = field.placeholder;
    control.maxLength = field.max;
    if (field.type === 'text') control.type = 'text';
  }
  const foot = document.createElement('div');
  foot.className = 'field-foot';
  const hint = document.createElement('p');
  hint.className = 'hint';
  hint.id = `${field.id}-hint`;
  hint.textContent = field.hint;
  foot.append(hint);
  if (field.max) {
    const counter = document.createElement('span');
    counter.className = 'counter';
    counter.id = `${field.id}-counter`;
    foot.append(counter);
  }
  wrapper.append(label, control, foot);
  document.querySelector(`#${field.group}-fields`).append(wrapper);
}

function getValues() {
  return Object.fromEntries(fields.map(field => [field.id, form.elements[field.id].value.trim()]));
}

// Прозрачная оценка полноты. Семантическую проверку можно добавить отдельно.
function calculateRating(values) {
  const breakdown = criteria.map(criterion => ({
    ...criterion,
    points: criterion.fields.every(id => Boolean(values[id])) ? criterion.weight : 0
  }));
  const score = breakdown.reduce((sum, item) => sum + item.points, 0);
  const level = score < 40 ? 'Черновик' : score < 70 ? 'Рабочая' : score < 90 ? 'Готовая' : 'Приоритетная';
  return { score, level, breakdown };
}

function renderRating() {
  const values = getValues();
  const rating = calculateRating(values);
  document.querySelector('#score').textContent = rating.score;
  document.querySelector('#progress').value = rating.score;
  document.querySelector('#level').textContent = rating.level;
  document.querySelector('#score-state').textContent = confirmation.checked ? 'Подтверждено вами' : 'Предварительная оценка';
  const container = document.querySelector('#breakdown');
  container.replaceChildren();
  for (const item of rating.breakdown) {
    const row = document.createElement('div');
    row.className = `criterion${item.points ? ' complete' : ''}`;
    const label = document.createElement('span');
    label.textContent = `${item.points ? '✓' : '○'} ${item.label}`;
    const points = document.createElement('strong');
    points.textContent = `${item.points} / ${item.weight}`;
    row.append(label, points);
    container.append(row);
  }
  const missing = rating.breakdown.find(item => !item.points);
  document.querySelector('#next-tip').textContent = missing
    ? `Заполните блок «${missing.label.toLowerCase()}»: +${missing.weight} баллов после подтверждения.`
    : 'Все блоки заполнены. Проверьте сведения и сформируйте карточку.';
  for (const field of fields.filter(item => item.max)) {
    document.querySelector(`#${field.id}-counter`).textContent = `${form.elements[field.id].value.length} / ${field.max}`;
  }
}

function saveDraft() {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(getValues()));
    document.querySelector('#save-status').textContent = 'Черновик сохранён в этом браузере.';
  } catch {
    document.querySelector('#save-status').textContent = 'Автосохранение недоступно. После заполнения скачайте карточку JSON.';
  }
}

function invalidateCard() {
  currentCard = null;
  document.querySelector('#preview').hidden = true;
}

form.addEventListener('input', event => {
  if (event.target !== confirmation) {
    confirmation.checked = false;
    if (event.target.name === 'title') event.target.setCustomValidity('');
    saveDraft();
  }
  invalidateCard();
  renderRating();
});

form.addEventListener('submit', event => {
  event.preventDefault();
  const values = getValues();
  if (!values.title) {
    form.elements.title.setCustomValidity('Введите название задачи, а не только пробелы.');
    form.elements.title.reportValidity();
    return;
  }
  if (!form.reportValidity()) return;
  const rating = calculateRating(values);
  currentCard = {
    schemaVersion: 1,
    ...values,
    status: 'confirmed',
    published: false,
    confirmedAt: new Date().toISOString(),
    rating: { score: rating.score, level: rating.level, breakdown: rating.breakdown },
    missingFields: fields.filter(field => !values[field.id]).map(field => field.id)
  };
  const container = document.querySelector('#preview-content');
  container.replaceChildren();
  const summary = document.createElement('p');
  summary.textContent = `${rating.score} / 100 · ${rating.level}`;
  const list = document.createElement('dl');
  for (const field of fields) {
    const term = document.createElement('dt');
    term.textContent = field.label;
    const description = document.createElement('dd');
    description.textContent = values[field.id] || 'Не указано';
    list.append(term, description);
  }
  container.append(summary, list);
  document.querySelector('#preview').hidden = false;
  document.querySelector('#preview-heading').focus();
  document.querySelector('#preview').scrollIntoView({ behavior: 'smooth', block: 'start' });
  // Точка интеграции: обработчик получает подтверждённую карточку.
  form.dispatchEvent(new CustomEvent('task:confirmed', { bubbles: true, detail: JSON.parse(JSON.stringify(currentCard)) }));
});

document.querySelector('#download').addEventListener('click', () => {
  if (!currentCard) return;
  const blob = new Blob([JSON.stringify(currentCard, null, 2)], { type: 'application/json;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = 'business-task.json';
  document.body.append(link);
  link.click();
  link.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
});

document.querySelector('#reset').addEventListener('click', () => {
  if (!window.confirm('Очистить все ответы и сохранённый черновик?')) return;
  form.reset();
  form.elements.title.setCustomValidity('');
  invalidateCard();
  try {
    localStorage.removeItem(STORAGE_KEY);
    document.querySelector('#save-status').textContent = 'Черновик очищен.';
  } catch {
    document.querySelector('#save-status').textContent = 'Поля очищены. Не удалось удалить сохранённый черновик из браузера.';
  }
  renderRating();
  form.elements.title.focus();
});

try {
  const saved = JSON.parse(localStorage.getItem(STORAGE_KEY) || 'null');
  if (saved && typeof saved === 'object') {
    for (const field of fields) {
      if (typeof saved[field.id] === 'string') {
        form.elements[field.id].value = field.max ? saved[field.id].slice(0, field.max) : saved[field.id];
      }
    }
    document.querySelector('#save-status').textContent = 'Восстановлен сохранённый черновик. Проверьте и подтвердите сведения.';
  }
} catch {
  document.querySelector('#save-status').textContent = 'Не удалось восстановить черновик. Можно заполнить форму заново.';
}
confirmation.checked = false;
renderRating();
