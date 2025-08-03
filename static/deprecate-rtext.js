// DOM Elements
const noteEditor = document.getElementById('note-editor');
const decreaseFontBtn = document.getElementById('decrease-font');
const increaseFontBtn = document.getElementById('increase-font');

// State Variables
let lastCaretPosition = null;
let baseFontSize = 16;
let lastSaveTime = Date.now();
let saveTimeout = null;
let isDirty = false;

// Initialize Application
document.addEventListener('DOMContentLoaded', function() {
  loadContent();
  setupEventListeners();
  updateFontSize();
});

// Load content from the backend
function loadContent() {
  fetch('/notepad/rtext.file')
    .then(response => {
      if (!response.ok) {
        throw new Error('Network response was not ok');
      }
      return response.text();
    })
    .then(content => {
      noteEditor.innerHTML = content;
    })
    .catch(error => {
      console.error('Error loading notepad content:', error);
    });
}

// Save content to the backend
function saveContent() {
  if (!isDirty) return;
  const content = noteEditor.innerHTML;
  fetch('/notepad/rtext.file', {
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

// Schedule an auto-save after inactivity
function scheduleAutoSave() {
  // Clear any existing timeout
  if (saveTimeout) {
    clearTimeout(saveTimeout);
  }
  saveTimeout = setTimeout(() => {
    saveContent();
  }, 2000);
}

// Basic Setup Functions
function setupEventListeners() {
  // Caret position tracking
  noteEditor.addEventListener('mouseup', updateCaretPosition);
  noteEditor.addEventListener('keyup', updateCaretPosition);
  noteEditor.addEventListener('focus', updateCaretPosition);
  // Auto-save events
  noteEditor.addEventListener('input', function() {
    isDirty = true;
    scheduleAutoSave();
  });
  noteEditor.addEventListener('keyup', function() {
    scheduleAutoSave();
  });
  // Save before leaving the page
  window.addEventListener('beforeunload', function() {
    if (isDirty) {
      saveContent();
    }
  });
  
  // Font size controls
  decreaseFontBtn.addEventListener('click', decreaseFontSize);
  increaseFontBtn.addEventListener('click', increaseFontSize);
  
  // Handle link clicks
  noteEditor.addEventListener('click', function(event) {
    const target = event.target;
    if (target.tagName === 'A') {
      event.preventDefault();
      window.open(target.href, '_blank');
    }
  });
  
  // Handle paste event to always paste as plain text
  noteEditor.addEventListener('paste', function(e) {
    e.preventDefault();
    const text = (e.clipboardData || window.clipboardData).getData('text');
    if (text) {
      document.execCommand('insertText', false, text);
      isDirty = true;
      scheduleAutoSave();
    }
  });
}

// Editor Formatting Functions
function formatDoc(cmd, value = null) {
  if (cmd === 'insertHorizontalRule') {
    document.execCommand(cmd, false, value);
    const selection = window.getSelection();
    if (selection.rangeCount > 0) {
      const range = selection.getRangeAt(0);
      let node = range.startContainer;
      // Navigate up to find the contenteditable element
      while (node && node !== noteEditor) {
        node = node.parentNode;
      }
      if (node) {
        // Find the most recently inserted HR element
        const hrs = node.querySelectorAll('hr');
        if (hrs.length > 0) {
          const lastHr = hrs[hrs.length - 1];
          lastHr.style.borderColor = '#a2c1f4';
          lastHr.style.backgroundColor = '#a2c1f4';
          lastHr.style.height = '1px';
        }
      }
    }
  } else {
    document.execCommand(cmd, false, value);
  }
  noteEditor.focus();
  isDirty = true;
  scheduleAutoSave();
}

function updateCaretPosition() {
  const sel = window.getSelection();
  if (sel.getRangeAt && sel.rangeCount) {
    lastCaretPosition = sel.getRangeAt(0);
  }
}

function createLink() {
  const selection = window.getSelection();
  if (!selection || selection.rangeCount === 0 || selection.toString().trim() === '') {
    alert('Please select some text first');
    return;
  }
  const range = selection.getRangeAt(0);
  const modal = document.createElement('div');
  modal.classList.add('modal');
  modal.innerHTML = `
    <div class="modal-content">
      <p>Add Link</p>
      <input type="url" id="urlInput" class="url-input" placeholder="https://example.com" value="https://example.com">
      <button id="confirm-link">Add</button>
      <button id="cancel-link">Cancel</button>
    </div>
  `;
  
  document.body.appendChild(modal);
  const urlInput = modal.querySelector('#urlInput');
  urlInput.focus();
  urlInput.select();
  
  function handleConfirm() {
    const url = urlInput.value;
    if (url) {
      formatDoc('createLink', url);
    }
    modal.remove();
  }
  
  modal.querySelector('#confirm-link').addEventListener('click', handleConfirm);
  modal.querySelector('#cancel-link').addEventListener('click', function() {
    modal.remove();
  });
  
  modal.addEventListener('click', function(e) {
    if (e.target === modal) {
      modal.remove();
    }
  });
  
  modal.addEventListener('keydown', function(e) {
    if (e.key === 'Enter') {
      handleConfirm();
    }
  });
}

function insertCodeBlock() {
  const modal = document.createElement('div');
  modal.className = 'modal';
  modal.innerHTML = `
    <div class="modal-content">
      <p>Add Code Block</p>
      <textarea rows="8" placeholder="Enter your code here"></textarea>
      <button id="confirm-code">Add</button>
      <button id="cancel-code">Cancel</button>
    </div>
  `;
  
  document.body.appendChild(modal);
  const textarea = modal.querySelector('textarea');
  textarea.focus();
  
  modal.querySelector('#confirm-code').addEventListener('click', function() {
    if (textarea.value.trim()) {
      const codeBlock = document.createElement('div');
      codeBlock.className = 'code-block';
      codeBlock.textContent = textarea.value;
      if (lastCaretPosition) {
        lastCaretPosition.deleteContents();
        lastCaretPosition.insertNode(codeBlock);
      } else {
        noteEditor.appendChild(codeBlock);
      }
      isDirty = true;
      scheduleAutoSave();
    }
    modal.remove();
  });
  
  modal.querySelector('#cancel-code').addEventListener('click', function() {
    modal.remove();
  });
  modal.addEventListener('click', function(e) {
    if (e.target === modal) {
      modal.remove();
    }
  });
}

function insertTable() {
  const modal = document.createElement('div');
  modal.className = 'modal';
  modal.innerHTML = `
    <div class="modal-content">
      <p>Select Table Size</p>
      <div class="grid"></div>
      <button id="cancel-table">Cancel</button>
    </div>
  `;
  
  document.body.appendChild(modal);
  const grid = modal.querySelector('.grid');
  const MAX_SIZE = 10;
  let selectedRows = 0;
  let selectedCols = 0;
  
  // Create grid cells
  for (let i = 1; i <= MAX_SIZE; i++) {
    for (let j = 1; j <= MAX_SIZE; j++) {
      const cell = document.createElement('div');
      cell.className = 'cell';
      cell.dataset.row = i;
      cell.dataset.col = j;
      grid.appendChild(cell);
    }
  }
  
  // Handle cell hover
  grid.addEventListener('mouseover', function(e) {
    if (e.target.classList.contains('cell')) {
      selectedRows = +e.target.dataset.row;
      selectedCols = +e.target.dataset.col;
      highlightCells(selectedRows, selectedCols);
    }
  });
  
  // Handle cell click
  grid.addEventListener('click', function(e) {
    if (e.target.classList.contains('cell')) {
      insertTableHTML(selectedRows, selectedCols);
      modal.remove();
    }
  });
  
  // Highlight cells in grid
  function highlightCells(rows, cols) {
    document.querySelectorAll('.cell').forEach(cell => {
      const cellRow = +cell.dataset.row;
      const cellCol = +cell.dataset.col;
      cell.classList.toggle('selected', cellRow <= rows && cellCol <= cols);
    });
  }
  
  // Insert table into editor
  function insertTableHTML(rows, cols) {
    const ZERO_WIDTH_SPACE = '\u200B';
    let tableHtml = '<table style="width: 100%;"><thead><tr>';
    
    // Create header cells
    for (let j = 0; j < cols; j++) {
      tableHtml += `<th>${ZERO_WIDTH_SPACE}</th>`;
    }
    tableHtml += '</tr></thead><tbody>';
    
    // Create rows and cells
    for (let i = 0; i < rows - 1; i++) {
      tableHtml += '<tr>';
      for (let j = 0; j < cols; j++) {
        tableHtml += `<td>${ZERO_WIDTH_SPACE}</td>`;
      }
      tableHtml += '</tr>';
    }
    
    tableHtml += '</tbody></table>';
    const tableElement = document.createElement('div');
    tableElement.innerHTML = tableHtml;
    if (lastCaretPosition) {
      lastCaretPosition.deleteContents();
      lastCaretPosition.insertNode(tableElement.firstChild);
      lastCaretPosition.collapse(false);
    } else {
      noteEditor.appendChild(tableElement.firstChild);
    }
    
    // Add resize handle to table
    const table = noteEditor.querySelector('table:last-of-type');
    table.style.position = 'relative';
    table.style.width = '100%';
    
    const resizeHandle = document.createElement('div');
    resizeHandle.className = 'resize-handle';
    resizeHandle.innerHTML = '↘';
    table.appendChild(resizeHandle);
    
    setupTableResizing(table, resizeHandle);
    isDirty = true;
    scheduleAutoSave();
  }
  
  modal.querySelector('#cancel-table').addEventListener('click', function() {
    modal.remove();
  });
}

// Table Resizing Functionality
function setupTableResizing(table, handle) {
  let isResizing = false;
  let startX, startY;
  let startWidth, startHeight;
  
  handle.addEventListener('mousedown', startResize);
  function startResize(e) {
    e.preventDefault();
    e.stopPropagation();
    
    isResizing = true;
    startX = e.clientX;
    startY = e.clientY;
    startWidth = table.offsetWidth;
    startHeight = table.offsetHeight;
    
    document.addEventListener('mousemove', resize);
    document.addEventListener('mouseup', stopResize);
  }
  
  function resize(e) {
    if (!isResizing) return;
    const width = startWidth + (e.clientX - startX);
    const height = startHeight + (e.clientY - startY);
    if (width > 100) {
      table.style.width = `${width}px`;
    }
    if (height > 50) {
      table.style.height = `${height}px`;
    }
  }
  
  function stopResize() {
    isResizing = false;
    document.removeEventListener('mousemove', resize);
    document.removeEventListener('mouseup', stopResize);
    isDirty = true;
    scheduleAutoSave();
  }
}

// Font Size Functionality
function decreaseFontSize() {
  if (baseFontSize > 10) {
    baseFontSize -= 1;
    updateFontSize();
  }
}

function increaseFontSize() {
  if (baseFontSize < 30) {
    baseFontSize += 1;
    updateFontSize();
  }
}

function updateFontSize() {
  noteEditor.style.fontSize = `${baseFontSize}px`;
  noteEditor.querySelectorAll("h1").forEach(h1 => {
    h1.style.fontSize = `${baseFontSize * 1.5}px`;
  });
  noteEditor.querySelectorAll("h2").forEach(h2 => {
    h2.style.fontSize = `${baseFontSize * 1.3}px`;
  });
  noteEditor.querySelectorAll("th, td").forEach(cell => {
    cell.style.fontSize = `${baseFontSize}px`;
  });
  noteEditor.querySelectorAll(".code-block").forEach(codeBlock => {
    codeBlock.style.fontSize = `${baseFontSize}px`;
  });
}

// Select All Text
function selectAllText() {
  const selection = window.getSelection();
  const range = document.createRange();
  range.selectNodeContents(noteEditor);
  selection.removeAllRanges();
  selection.addRange(range);
}
