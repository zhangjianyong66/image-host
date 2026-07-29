const form = document.querySelector('#login-form');
const passwordField = document.querySelector('#password');
const message = document.querySelector('#login-message');
const submitButton = document.querySelector('#login-button');
const toggleButton = document.querySelector('#toggle-password');

toggleButton.addEventListener('click', () => {
  const revealing = passwordField.type === 'password';
  passwordField.type = revealing ? 'text' : 'password';
  toggleButton.setAttribute('aria-label', revealing ? '隐藏密码' : '显示密码');
  toggleButton.setAttribute('title', revealing ? '隐藏密码' : '显示密码');
  toggleButton.classList.toggle('is-visible', revealing);
  passwordField.focus();
});

form.addEventListener('submit', async (event) => {
  event.preventDefault();
  message.textContent = '';
  if (!passwordField.value) {
    message.textContent = '请输入访问密码';
    passwordField.focus();
    return;
  }

  submitButton.disabled = true;
  submitButton.firstElementChild.textContent = '正在验证';
  try {
    const response = await fetch('/api/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ['password']: passwordField.value }),
    });
    const body = await response.json().catch(() => ({}));
    if (!response.ok) {
      throw new Error(body.error || '登录失败，请稍后重试');
    }
    window.location.assign('/upload');
  } catch (error) {
    message.textContent = error.message;
    passwordField.select();
  } finally {
    submitButton.disabled = false;
    submitButton.firstElementChild.textContent = '进入上传台';
  }
});
