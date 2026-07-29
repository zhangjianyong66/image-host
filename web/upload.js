const MAX_IMAGE_BYTES = 10 * 1024 * 1024;
const ALLOWED_TYPES = new Set(['image/jpeg', 'image/png', 'image/gif', 'image/webp']);

const fileInput = document.querySelector('#file-input');
const dropZone = document.querySelector('#drop-zone');
const selection = document.querySelector('#selection');
const preview = document.querySelector('#preview');
const fileName = document.querySelector('#file-name');
const fileMeta = document.querySelector('#file-meta');
const uploadButton = document.querySelector('#upload-button');
const replaceButton = document.querySelector('#replace-button');
const uploadProgress = document.querySelector('#upload-progress');
const progressValue = document.querySelector('#progress-value');
const progressBar = document.querySelector('#progress-bar');
const result = document.querySelector('#result');
const resultImage = document.querySelector('#result-image');
const resultURL = document.querySelector('#result-url');
const copyButton = document.querySelector('#copy-button');
const copyMessage = document.querySelector('#copy-message');
const uploadAnother = document.querySelector('#upload-another');
const uploadMessage = document.querySelector('#upload-message');
const logoutButton = document.querySelector('#logout-button');

let selectedFile = null;
let previewURL = '';

function formatBytes(bytes) {
  if (bytes >= 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MiB`;
  return `${Math.max(1, Math.round(bytes / 1024))} KiB`;
}

function resetPreviewURL() {
  if (previewURL) URL.revokeObjectURL(previewURL);
  previewURL = '';
}

function rejectFile(message) {
  selectedFile = null;
  fileInput.value = '';
  resetPreviewURL();
  selection.hidden = true;
  result.hidden = true;
  dropZone.hidden = false;
  uploadMessage.textContent = message;
}

function selectFile(file) {
  uploadMessage.textContent = '';
  if (!file || !ALLOWED_TYPES.has(file.type)) {
    rejectFile('请选择 JPEG、PNG、GIF 或 WebP 图片');
    return;
  }
  if (file.size === 0) {
    rejectFile('图片文件不能为空');
    return;
  }
  if (file.size > MAX_IMAGE_BYTES) {
    rejectFile('图片不能超过 10 MiB');
    return;
  }

  selectedFile = file;
  resetPreviewURL();
  previewURL = URL.createObjectURL(file);
  preview.src = previewURL;
  fileName.textContent = file.name;
  fileMeta.textContent = `${file.type.replace('image/', '').toUpperCase()} · ${formatBytes(file.size)}`;
  dropZone.hidden = true;
  result.hidden = true;
  selection.hidden = false;
}

dropZone.addEventListener('click', () => fileInput.click());
replaceButton.addEventListener('click', () => fileInput.click());
fileInput.addEventListener('change', () => selectFile(fileInput.files[0]));

for (const eventName of ['dragenter', 'dragover']) {
  dropZone.addEventListener(eventName, (event) => {
    event.preventDefault();
    dropZone.classList.add('is-dragging');
  });
}
for (const eventName of ['dragleave', 'drop']) {
  dropZone.addEventListener(eventName, (event) => {
    event.preventDefault();
    dropZone.classList.remove('is-dragging');
  });
}
dropZone.addEventListener('drop', (event) => selectFile(event.dataTransfer.files[0]));

uploadButton.addEventListener('click', () => {
  if (!selectedFile) return;
  uploadMessage.textContent = '';
  selection.hidden = true;
  uploadProgress.hidden = false;
  uploadButton.disabled = true;
  progressValue.textContent = '0%';
  progressBar.style.width = '0%';

  const formData = new FormData();
  formData.append('image', selectedFile);
  const request = new XMLHttpRequest();
  request.open('POST', '/api/images');
  request.upload.addEventListener('progress', (event) => {
    if (!event.lengthComputable) return;
    const percent = Math.round((event.loaded / event.total) * 100);
    progressValue.textContent = `${percent}%`;
    progressBar.style.width = `${percent}%`;
  });
  request.addEventListener('load', () => {
    uploadProgress.hidden = true;
    uploadButton.disabled = false;
    let body = {};
    try {
      body = JSON.parse(request.responseText || '{}');
    } catch {
      body = {};
    }
    if (request.status === 401) {
      window.location.assign('/login');
      return;
    }
    if (request.status !== 201) {
      selection.hidden = false;
      uploadMessage.textContent = body.error || '上传失败，请稍后重试';
      return;
    }
    resultURL.value = body.url;
    resultImage.src = body.url;
    result.hidden = false;
    copyMessage.textContent = '';
  });
  request.addEventListener('error', () => {
    uploadProgress.hidden = true;
    uploadButton.disabled = false;
    selection.hidden = false;
    uploadMessage.textContent = '网络连接失败，请重试';
  });
  request.send(formData);
});

copyButton.addEventListener('click', async () => {
  copyMessage.textContent = '';
  try {
    await navigator.clipboard.writeText(resultURL.value);
    copyMessage.textContent = '链接已复制到剪贴板';
  } catch {
    resultURL.focus();
    resultURL.select();
    const copied = document.execCommand('copy');
    copyMessage.textContent = copied ? '链接已复制到剪贴板' : '请手动复制已选中的链接';
  }
});

uploadAnother.addEventListener('click', () => {
  selectedFile = null;
  fileInput.value = '';
  resetPreviewURL();
  result.hidden = true;
  selection.hidden = true;
  dropZone.hidden = false;
  uploadMessage.textContent = '';
  dropZone.focus();
});

logoutButton.addEventListener('click', async () => {
  logoutButton.disabled = true;
  try {
    await fetch('/api/logout', { method: 'POST' });
  } finally {
    window.location.assign('/login');
  }
});
