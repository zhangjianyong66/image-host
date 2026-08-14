const MAX_IMAGE_BYTES = 10 * 1024 * 1024;
const MAX_DOCUMENT_BYTES = 50 * 1024 * 1024;
const FILE_TYPES = new Map([
  ['jpg', { mime: 'image/jpeg', label: 'JPEG', image: true }],
  ['jpeg', { mime: 'image/jpeg', label: 'JPEG', image: true }],
  ['png', { mime: 'image/png', label: 'PNG', image: true }],
  ['gif', { mime: 'image/gif', label: 'GIF', image: true }],
  ['webp', { mime: 'image/webp', label: 'WEBP', image: true }],
  ['pdf', { mime: 'application/pdf', label: 'PDF', image: false }],
  ['epub', { mime: 'application/epub+zip', label: 'EPUB', image: false }],
  ['txt', { mime: 'text/plain', label: 'TXT', image: false }],
  ['md', { mime: 'text/markdown', label: 'MD', image: false }],
  ['docx', { mime: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document', label: 'DOCX', image: false }],
  ['xlsx', { mime: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet', label: 'XLSX', image: false }],
  ['pptx', { mime: 'application/vnd.openxmlformats-officedocument.presentationml.presentation', label: 'PPTX', image: false }],
]);

const fileInput = document.querySelector('#file-input');
const dropZone = document.querySelector('#drop-zone');
const selection = document.querySelector('#selection');
const preview = document.querySelector('#preview');
const selectionDocument = document.querySelector('#selection-document');
const selectionDocumentType = document.querySelector('#selection-document-type');
const fileName = document.querySelector('#file-name');
const fileMeta = document.querySelector('#file-meta');
const uploadButton = document.querySelector('#upload-button');
const replaceButton = document.querySelector('#replace-button');
const uploadProgress = document.querySelector('#upload-progress');
const progressValue = document.querySelector('#progress-value');
const progressBar = document.querySelector('#progress-bar');
const result = document.querySelector('#result');
const resultImage = document.querySelector('#result-image');
const resultDocument = document.querySelector('#result-document');
const resultDocumentType = document.querySelector('#result-document-type');
const resultURL = document.querySelector('#result-url');
const copyButton = document.querySelector('#copy-button');
const copyMessage = document.querySelector('#copy-message');
const uploadAnother = document.querySelector('#upload-another');
const uploadMessage = document.querySelector('#upload-message');
const logoutButton = document.querySelector('#logout-button');

let selectedFile = null;
let selectedType = null;
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
  selectedType = null;
  fileInput.value = '';
  resetPreviewURL();
  selection.hidden = true;
  result.hidden = true;
  dropZone.hidden = false;
  uploadMessage.textContent = message;
}

function typeForFile(file) {
  const extension = file.name.split('.').pop().toLowerCase();
  return FILE_TYPES.get(extension) || null;
}

function fileSizeLimit(type) {
  return type.image ? MAX_IMAGE_BYTES : MAX_DOCUMENT_BYTES;
}

function selectFile(file) {
  uploadMessage.textContent = '';
  const type = file && typeForFile(file);
  if (!type) {
    rejectFile('请选择支持的图片或文档文件');
    return;
  }
  if (file.size === 0) {
    rejectFile('文件不能为空');
    return;
  }
  if (file.size > fileSizeLimit(type)) {
    rejectFile(type.image ? '图片不能超过 10 MiB' : '文档不能超过 50 MiB');
    return;
  }

  selectedFile = file;
  selectedType = type;
  resetPreviewURL();
  if (type.image) {
    previewURL = URL.createObjectURL(file);
    preview.src = previewURL;
    preview.hidden = false;
    selectionDocument.hidden = true;
  } else {
    preview.removeAttribute('src');
    preview.hidden = true;
    selectionDocumentType.textContent = type.label;
    selectionDocument.hidden = false;
  }
  fileName.textContent = file.name;
  fileMeta.textContent = `${type.label} · ${formatBytes(file.size)}`;
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
  if (!selectedFile || !selectedType) return;
  uploadMessage.textContent = '';
  selection.hidden = true;
  uploadProgress.hidden = false;
  uploadButton.disabled = true;
  progressValue.textContent = '0%';
  progressBar.style.width = '0%';

  const formData = new FormData();
  formData.append('image', new File([selectedFile], selectedFile.name, { type: selectedType.mime }));
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
    if (selectedType.image) {
      resultImage.src = body.url;
      resultImage.hidden = false;
      resultDocument.hidden = true;
    } else {
      resultImage.removeAttribute('src');
      resultImage.hidden = true;
      resultDocumentType.textContent = selectedType.label;
      resultDocument.hidden = false;
    }
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
  selectedType = null;
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
