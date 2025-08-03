// DOM Elements
const markdownEditor = document.getElementById('markdown-editor');
const markdownPreview = document.getElementById('markdown-preview');
const toggleModeBtn = document.getElementById('toggle-mode');
const editorContainer = document.getElementById('editor-container');

// State Variables
let undoStack = [];
let redoStack = [];
let lastChange = '';
let isReaderMode = false;
let lastSaveTime = Date.now();
let saveTimeout = null;
let isDirty = false;

// Initialize Application
document.addEventListener('DOMContentLoaded', function() {
  loadContent();
  setupEventListeners();
});

// Save state and schedule auto-save on input
function setupEventListeners() {
  markdownEditor.addEventListener('input', function() {
    saveState();
    isDirty = true;
    scheduleAutoSave();
  });
  markdownEditor.addEventListener('keyup', scheduleAutoSave);
  // Ensure content is saved before leaving the page
  window.addEventListener('beforeunload', function() {
    if (isDirty) {
      saveContent();
    }
  });
  
  // Handle tab key for indentation in the editor
  markdownEditor.addEventListener('keydown', function(e) {
    if (e.key === 'Tab') {
      e.preventDefault();
      const start = this.selectionStart;
      const end = this.selectionEnd;
      this.value = this.value.substring(0, start) + '  ' + this.value.substring(end);
      this.selectionStart = this.selectionEnd = start + 2;
      saveState();
      isDirty = true;
      scheduleAutoSave();
    }
  });
}

// Load notepad content from the backend
function loadContent() {
  fetch('/notepad/md.file')
    .then(response => {
      if (!response.ok) {
        throw new Error('Network response was not ok');
      }
      return response.text();
    })
    .then(content => {
      markdownEditor.value = content;
      saveState(); // Initialize the undo/redo stacks with initial content
    })
    .catch(error => {
      console.error('Error loading notepad content:', error);
      saveState();
    });
}

// Save notepad content to the backend
function saveContent() {
  if (!isDirty) return; // Don't save if there are no changes
  const content = markdownEditor.value;
  fetch('/notepad/md.file', {
    method: 'POST',
    body: content,
    headers: {
      'Content-Type': 'text/plain'
    }
  })
  .then(response => {
    if (!response.ok) {
      throw new Error('Network response was not ok');
    }
    lastSaveTime = Date.now();
    isDirty = false;
    console.log('Content saved successfully');
  })
  .catch(error => {
    console.error('Error saving content:', error);
  });
}

// Schedule an auto-save after a period of inactivity
function scheduleAutoSave() {
  if (saveTimeout) {
    clearTimeout(saveTimeout);
  }
  saveTimeout = setTimeout(() => {
    saveContent();
  }, 2000);
}

// Update the preview pane with rendered markdown
function updatePreview() {
    marked.setOptions({
      breaks: true,
      gfm: true,
      headerIds: true,
      highlight: function(code, language) {
        if (language && hljs.getLanguage(language)) {
          try {
            return hljs.highlight(code, { language: language }).value;
          } catch (err) {
            console.error('Highlighting error:', err);
          }
        }
        return hljs.highlightAuto(code).value;
      }
    });
    markdownPreview.innerHTML = marked.parse(markdownEditor.value);
    markdownPreview.querySelectorAll('pre code').forEach((block) => {
      hljs.highlightElement(block);
    });
    const links = markdownPreview.querySelectorAll('a');
    links.forEach(link => {
      link.setAttribute('target', 'blank');
      link.setAttribute('rel', 'noopener noreferrer');
    });
  }

// Toggle between reader (preview-only) and writer (editor) mode
function toggleMode() {
  isReaderMode = !isReaderMode;
  if (isReaderMode) {
    updatePreview();
    markdownEditor.classList.add('hidden');
    markdownPreview.classList.remove('hidden');
    toggleModeBtn.innerHTML = '<i class="fas fa-pencil-alt text-subtext0"></i>';
    toggleModeBtn.title = "Write Mode";
  } else {
    markdownEditor.classList.remove('hidden');
    markdownPreview.classList.add('hidden');
    toggleModeBtn.innerHTML = '<i class="fas fa-book-reader text-subtext0"></i>';
    toggleModeBtn.title = "Reader Mode";
    markdownEditor.focus();
  }
}

function saveState() {
  const currentValue = markdownEditor.value;
  if (currentValue !== lastChange) {
    undoStack.push(lastChange);
    lastChange = currentValue;
    redoStack = [];
    if (undoStack.length > 100) {
      undoStack.shift();
    }
  }
}

function undoChange() {
  if (undoStack.length > 0) {
    const currentValue = markdownEditor.value;
    redoStack.push(currentValue);
    const previousValue = undoStack.pop();
    markdownEditor.value = previousValue;
    lastChange = previousValue;
    isDirty = true;
    scheduleAutoSave();
  }
}

function redoChange() {
  if (redoStack.length > 0) {
    const currentValue = markdownEditor.value;
    undoStack.push(currentValue);
    const nextValue = redoStack.pop();
    markdownEditor.value = nextValue;
    lastChange = nextValue;
    isDirty = true;
    scheduleAutoSave();
  }
}
