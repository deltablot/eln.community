document.addEventListener('DOMContentLoaded', function () {

  const errorDialog = document.getElementById('error-dialog');
  if (errorDialog) {
    const closeButton = errorDialog.querySelector("button");
    if (closeButton) {
      closeButton.addEventListener("click", () => {
        errorDialog.close();
      });
    }
  }

  const tosLink = document.getElementById('tos-link');
  const tosDialog = document.getElementById('tos');
  if (tosLink && tosDialog) {
    tosLink.addEventListener("click", () => {
      tosDialog.showModal();
    });
    const closeButtonTos = tosDialog.querySelector("button");
    if (closeButtonTos) {
      closeButtonTos.addEventListener("click", () => {
        tosDialog.close();
      });
    }
  }

  // Handle form submission for success toast
  const uploadForm = document.querySelector('form[action="/api/v1/records"]');
  if (uploadForm) {
    const submitBtn = document.getElementById('submitBtn');
    const spinner = submitBtn?.querySelector('.spinner-border');
    const btnText = submitBtn?.querySelector('.btn-text');

    uploadForm.addEventListener('submit', async function (e) {
      e.preventDefault();

      if (submitBtn) {
        submitBtn.disabled = true;
        spinner?.classList.remove('d-none');
        if (btnText) btnText.textContent = 'Uploading...';
      }

      const formData = new FormData(this);

      try {
        const response = await fetch('/api/v1/records', {
          method: 'POST',
          body: formData
        });

        if (response.ok) {
          // Show success toast
          const successToast = document.getElementById('successToast');
          if (successToast) {
            const toast = new bootstrap.Toast(successToast, {
              delay: 3000,
              autohide: true
            });

            // Redirect to browse page after toast hides
            successToast.addEventListener('hidden.bs.toast', function () {
              window.location.href = '/browse';
            }, { once: true });

            toast.show();
          }

          this.reset();
        } else {
          // Show error toast
          const errorText = await response.text();
          const errorToast = document.getElementById('errorToast');
          const errorToastBody = document.getElementById('errorToastBody');
          if (errorToast && errorToastBody) {
            errorToastBody.textContent = errorText || 'Upload failed. Please try again.';
            const toast = new bootstrap.Toast(errorToast);
            toast.show();
          }
          console.error('Upload failed:', errorText);
        }
      } catch (error) {
        // Show error toast for network/other errors
        const errorToast = document.getElementById('errorToast');
        const errorToastBody = document.getElementById('errorToastBody');
        if (errorToast && errorToastBody) {
          errorToastBody.textContent = 'Network error. Please check your connection and try again.';
          const toast = new bootstrap.Toast(errorToast);
          toast.show();
        }
        console.error('Upload error:', error);
      } finally {
        // Reset button state
        if (submitBtn) {
          submitBtn.disabled = false;
          spinner?.classList.add('d-none');
          if (btnText) btnText.textContent = 'Send';
        }
      }
    });
  }

  // Initialize RO-Crate viewer if elements exist
  initializeRoCrateViewer();

  // Handle category select change for browse page
  const categorySelect = document.getElementById('category');
  const searchForm = document.getElementById('searchForm');
  if (categorySelect && searchForm) {
    categorySelect.addEventListener('change', function() {
      searchForm.submit();
    });
  }
});

// RO-Crate viewer functionality
let roCrateData = null;

function showFormattedView() {
  const formattedView = document.getElementById('formatted-view');
  const rawView = document.getElementById('raw-view');
  const formattedBtn = document.getElementById('view-formatted');
  const rawBtn = document.getElementById('view-raw');
  
  if (formattedView && rawView && formattedBtn && rawBtn) {
    formattedView.classList.remove('hidden');
    rawView.classList.add('hidden');
    formattedBtn.className = 'btn btn-primary btn-sm';
    rawBtn.className = 'btn btn-outline-secondary btn-sm';
  }
}

function showRawView() {
  const formattedView = document.getElementById('formatted-view');
  const rawView = document.getElementById('raw-view');
  const formattedBtn = document.getElementById('view-formatted');
  const rawBtn = document.getElementById('view-raw');
  
  if (formattedView && rawView && formattedBtn && rawBtn) {
    formattedView.classList.add('hidden');
    rawView.classList.remove('hidden');
    formattedBtn.className = 'btn btn-outline-secondary btn-sm';
    rawBtn.className = 'btn btn-primary btn-sm';
  }
}

function escapeHtml(text) {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

function isValidUrl(string) {
  try {
    const url = new URL(string);
    // Only allow http and https protocols for security
    return url.protocol === 'http:' || url.protocol === 'https:';
  } catch (_) {
    return false;
  }
}

function renderProperty(key, value, entity) {
  if (value === null || value === undefined) return '';

  let html = `<div class="ro-crate-property">
    <span class="ro-crate-property-name">${escapeHtml(key)}:</span>`;

  if (Array.isArray(value)) {
    html += '<div class="ro-crate-array">';
    value.forEach(item => {
      html += `<div class="ro-crate-array-item">${renderValue(item, entity, key)}</div>`;
    });
    html += '</div>';
  } else {
    html += renderValue(value, entity, key);
  }

  html += '</div>';
  return html;
}

function renderValue(value, entity, key) {
  if (value === null || value === undefined) return '<em>null</em>';

  if (typeof value === 'string') {
    // Check if it's a URL (with better validation for XSS protection)
    if (isValidUrl(value)) {
      return `<a href="${escapeHtml(value)}" class="ro-crate-link" target="_blank" rel="noopener noreferrer">${escapeHtml(value)}</a>`;
    }
    // Check if it's an ID reference
    if (value.startsWith('./') || value.startsWith('#')) {
      return `<span class="ro-crate-id">${escapeHtml(value)}</span>`;
    }

    // Check if this is HTML content based on encodingFormat
    if (entity && entity.encodingFormat === 'text/html' && (key === 'text' || key === 'description' || key === 'content')) {
      return renderHtmlContent(value, entity['@id']);
    }

    return escapeHtml(value);
  }

  if (typeof value === 'object') {
    if (value['@id']) {
      let html = `<span class="ro-crate-id">${escapeHtml(value['@id'])}</span>`;
      if (value['@type']) {
        const types = Array.isArray(value['@type']) ? value['@type'] : [value['@type']];
        types.forEach(type => {
          html += ` <span class="ro-crate-type-badge">${escapeHtml(type)}</span>`;
        });
      }
      return html;
    }
    return `<pre class="small">${escapeHtml(JSON.stringify(value, null, 2))}</pre>`;
  }

  return escapeHtml(String(value));
}

function renderHtmlContent(htmlContent, entityId) {
  // Create a safe blob URL for HTML content
  const safeEntityId = escapeHtml(entityId || 'unknown');
  const truncatedContent = htmlContent.length > 200 ? htmlContent.substring(0, 200) + '...' : htmlContent;
  const base64Content = btoa(unescape(encodeURIComponent(htmlContent)));

  return `
    <div class="html-content-preview">
      <div class="d-flex align-items-center gap-2 mb-2">
        <span class="badge bg-info">HTML Content</span>
        <button class="btn btn-sm btn-outline-primary html-open-btn" data-html-content="${base64Content}" data-entity-id="${safeEntityId}">
          <i class="fas fa-external-link-alt"></i> Open in New Tab
        </button>
      </div>
      <div class="html-preview bg-light p-2 rounded small">
        ${escapeHtml(truncatedContent)}
      </div>
    </div>
  `;
}

function openHtmlInNewTab(base64Content, entityId) {
  try {
    const htmlContent = decodeURIComponent(escape(atob(base64Content)));
    const blob = new Blob([htmlContent], { type: 'text/html' });
    const url = URL.createObjectURL(blob);
    const newWindow = window.open(url, '_blank');

    // Clean up the blob URL after a short delay
    setTimeout(() => {
      URL.revokeObjectURL(url);
    }, 1000);

    if (!newWindow) {
      alert('Please allow popups to view HTML content in a new tab.');
    }
  } catch (error) {
    console.error('Error opening HTML content:', error);
    alert('Error opening HTML content: ' + error.message);
  }
}

function renderEntity(entity) {
  let html = '<div class="ro-crate-entity">';

  // Header with ID and type
  html += '<div class="ro-crate-entity-header">';
  html += `<strong>ID:</strong> <span class="ro-crate-id">${escapeHtml(entity['@id'] || 'Unknown')}</span>`;

  if (entity['@type']) {
    html += ' ';
    const types = Array.isArray(entity['@type']) ? entity['@type'] : [entity['@type']];
    types.forEach(type => {
      html += `<span class="ro-crate-type-badge">${escapeHtml(type)}</span>`;
    });
  }
  html += '</div>';

  // Body with properties
  html += '<div class="ro-crate-entity-body">';

  // Sort properties, putting common ones first
  const commonProps = ['name', 'description', 'author', 'dateCreated', 'dateModified', 'license', 'url'];
  const sortedKeys = Object.keys(entity).sort((a, b) => {
    if (a === '@id' || a === '@type') return -1;
    if (b === '@id' || b === '@type') return 1;

    const aIndex = commonProps.indexOf(a);
    const bIndex = commonProps.indexOf(b);

    if (aIndex !== -1 && bIndex !== -1) return aIndex - bIndex;
    if (aIndex !== -1) return -1;
    if (bIndex !== -1) return 1;

    return a.localeCompare(b);
  });

  sortedKeys.forEach(key => {
    if (key !== '@id' && key !== '@type') {
      html += renderProperty(key, entity[key], entity);
    }
  });

  html += '</div></div>';
  return html;
}

function renderRoCrate(data) {
  if (!data || typeof data !== 'object') {
    return '<div class="alert alert-warning">Invalid RO-Crate format: data is not an object</div>';
  }

  if (!data['@graph'] || !Array.isArray(data['@graph'])) {
    return '<div class="alert alert-warning">Invalid RO-Crate format: missing or invalid @graph property</div>';
  }

  let html = '';

  // Find the root dataset (usually has @id of "./" or similar)
  const rootDataset = data['@graph'].find(entity => {
    if (!entity || typeof entity !== 'object') return false;
    if (entity['@id'] === './') return true;

    // Check if @type contains Dataset
    if (entity['@type']) {
      const types = Array.isArray(entity['@type']) ? entity['@type'] : [entity['@type']];
      return types.includes('Dataset');
    }

    return false;
  });

  if (rootDataset) {
    html += '<h4>Dataset Information</h4>';
    html += renderEntity(rootDataset);
  }

  // Group other entities by type
  const otherEntities = data['@graph'].filter(entity => entity !== rootDataset);
  const entityGroups = {};

  otherEntities.forEach(entity => {
    if (!entity || typeof entity !== 'object') return;

    const types = Array.isArray(entity['@type']) ? entity['@type'] : [entity['@type'] || 'Unknown'];
    const primaryType = types[0] || 'Unknown';

    if (!entityGroups[primaryType]) {
      entityGroups[primaryType] = [];
    }
    entityGroups[primaryType].push(entity);
  });

  // Render grouped entities
  Object.keys(entityGroups).sort().forEach(type => {
    if (entityGroups[type].length > 0) {
      html += `<h4>${escapeHtml(type)} (${entityGroups[type].length})</h4>`;
      entityGroups[type].forEach(entity => {
        html += renderEntity(entity);
      });
    }
  });

  return html;
}

// Initialize RO-Crate viewer functionality
function initializeRoCrateViewer() {
  // Only initialize if we're on a page with RO-Crate content
  const contentDiv = document.getElementById('ro-crate-content');
  if (!contentDiv) {
    return; // Not on a record page, skip initialization
  }

  // Add event listeners for view toggle buttons
  const formattedBtn = document.getElementById('view-formatted');
  const rawBtn = document.getElementById('view-raw');
  
  if (formattedBtn) {
    formattedBtn.addEventListener('click', showFormattedView);
  }
  
  if (rawBtn) {
    rawBtn.addEventListener('click', showRawView);
  }

  // Add event delegation for HTML open buttons (they are created dynamically)
  contentDiv.addEventListener('click', function(e) {
    if (e.target.classList.contains('html-open-btn') || e.target.closest('.html-open-btn')) {
      const button = e.target.classList.contains('html-open-btn') ? e.target : e.target.closest('.html-open-btn');
      const base64Content = button.getAttribute('data-html-content');
      const entityId = button.getAttribute('data-entity-id');
      
      if (base64Content && entityId) {
        openHtmlInNewTab(base64Content, entityId);
      }
    }
  });

  try {
    // Parse JSON data from the script tag
    const jsonDataElement = document.getElementById('ro-crate-json-data');
    if (!jsonDataElement) {
      throw new Error('RO-Crate data element not found');
    }

    const jsonText = jsonDataElement.textContent;
    if (!jsonText || jsonText.trim() === '') {
      throw new Error('RO-Crate data is empty');
    }

    roCrateData = JSON.parse(jsonText);

    contentDiv.innerHTML = renderRoCrate(roCrateData);
  } catch (error) {
    console.error('Error processing RO-Crate data:', error);
    const errorMessage = error instanceof Error ? error.message : String(error);
    contentDiv.innerHTML =
      '<div class="alert alert-danger">Error processing RO-Crate metadata: ' + escapeHtml(errorMessage) +
      '<br><small>Check browser console for more details</small></div>';
  }
}

// Handle edit form submission
function initializeEditForm() {
  const editForm = document.querySelector('form.edit-record-form');
  if (!editForm) {
    return; // Not on edit page
  }

  editForm.addEventListener('submit', async function (e) {
    e.preventDefault();

    const submitBtn = this.querySelector('button[type="submit"]');
    const originalText = submitBtn?.textContent;

    if (submitBtn) {
      submitBtn.disabled = true;
      submitBtn.textContent = 'Updating...';
    }

    // Convert form data to URL-encoded format
    const formData = new FormData(this);
    const urlEncodedData = new URLSearchParams(formData).toString();

    try {
      const response = await fetch(this.action, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/x-www-form-urlencoded',
        },
        body: urlEncodedData
      });

      if (response.ok) {
        // Show success toast
        const successToast = document.getElementById('successToast');
        const successToastBody = successToast?.querySelector('.toast-body');
        if (successToast && successToastBody) {
          successToastBody.textContent = 'Entry updated successfully!';
          const toast = new bootstrap.Toast(successToast, {
            delay: 2000,
            autohide: true
          });

          // Redirect to record page after toast hides
          const recordId = this.action.split('/').pop();
          successToast.addEventListener('hidden.bs.toast', function () {
            window.location.href = '/record/' + recordId;
          }, { once: true });

          toast.show();
        } else {
          // Fallback redirect if no toast
          const recordId = this.action.split('/').pop();
          setTimeout(() => {
            window.location.href = '/record/' + recordId;
          }, 1000);
        }
      } else {
        // Show error toast
        const errorText = await response.text();
        const errorToast = document.getElementById('errorToast');
        const errorToastBody = document.getElementById('errorToastBody');
        if (errorToast && errorToastBody) {
          errorToastBody.textContent = errorText || 'Update failed. Please try again.';
          const toast = new bootstrap.Toast(errorToast);
          toast.show();
        }
        console.error('Update failed:', errorText);
        
        // Reset button state
        if (submitBtn) {
          submitBtn.disabled = false;
          submitBtn.textContent = originalText || 'Update Entry';
        }
      }
    } catch (error) {
      // Show error toast for network/other errors
      const errorToast = document.getElementById('errorToast');
      const errorToastBody = document.getElementById('errorToastBody');
      if (errorToast && errorToastBody) {
        errorToastBody.textContent = 'Network error. Please check your connection and try again.';
        const toast = new bootstrap.Toast(errorToast);
        toast.show();
      }
      console.error('Update error:', error);
      
      // Reset button state
      if (submitBtn) {
        submitBtn.disabled = false;
        submitBtn.textContent = originalText || 'Update Entry';
      }
    }
  });
}

// Handle delete button click
function initializeDeleteButton() {
  const deleteBtn = document.getElementById('deleteBtn');
  const deleteForm = document.querySelector('form.delete-record-form');
  const deleteModal = document.getElementById('deleteConfirmModal');
  const confirmDeleteBtn = document.getElementById('confirmDeleteBtn');
  
  if (!deleteBtn || !deleteForm || !deleteModal || !confirmDeleteBtn) {
    return; // Not on edit page
  }

  // Show modal when delete button is clicked
  deleteBtn.addEventListener('click', function (e) {
    e.preventDefault();
    const modal = new bootstrap.Modal(deleteModal);
    modal.show();
  });

  // Handle actual delete when confirmed
  confirmDeleteBtn.addEventListener('click', async function (e) {
    e.preventDefault();

    const modal = bootstrap.Modal.getInstance(deleteModal);
    const originalText = confirmDeleteBtn.textContent;
    confirmDeleteBtn.disabled = true;
    confirmDeleteBtn.textContent = 'Deleting...';

    // Convert form data to URL-encoded format
    const formData = new FormData(deleteForm);
    const urlEncodedData = new URLSearchParams(formData).toString();

    try {
      const response = await fetch(deleteForm.action, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/x-www-form-urlencoded',
        },
        body: urlEncodedData
      });

      if (response.ok) {
        // Hide modal
        modal.hide();
        
        // Show success toast
        const successToast = document.getElementById('successToast');
        const successToastBody = successToast?.querySelector('.toast-body');
        if (successToast && successToastBody) {
          successToastBody.textContent = 'Entry deleted successfully!';
          const toast = new bootstrap.Toast(successToast, {
            delay: 2000,
            autohide: true
          });

          // Redirect to browse page after toast hides
          successToast.addEventListener('hidden.bs.toast', function () {
            window.location.href = '/browse';
          }, { once: true });

          toast.show();
        } else {
          // Fallback redirect if no toast
          setTimeout(() => {
            window.location.href = '/browse';
          }, 1000);
        }
      } else {
        // Hide modal
        modal.hide();
        
        // Show error toast
        const errorText = await response.text();
        const errorToast = document.getElementById('errorToast');
        const errorToastBody = document.getElementById('errorToastBody');
        if (errorToast && errorToastBody) {
          errorToastBody.textContent = errorText || 'Delete failed. Please try again.';
          const toast = new bootstrap.Toast(errorToast);
          toast.show();
        }
        console.error('Delete failed:', errorText);
        
        // Reset button state
        confirmDeleteBtn.disabled = false;
        confirmDeleteBtn.textContent = originalText;
      }
    } catch (error) {
      // Hide modal
      modal.hide();
      
      // Show error toast for network/other errors
      const errorToast = document.getElementById('errorToast');
      const errorToastBody = document.getElementById('errorToastBody');
      if (errorToast && errorToastBody) {
        errorToastBody.textContent = 'Network error. Please check your connection and try again.';
        const toast = new bootstrap.Toast(errorToast);
        toast.show();
      }
      console.error('Delete error:', error);
      
      // Reset button state
      confirmDeleteBtn.disabled = false;
      confirmDeleteBtn.textContent = originalText;
    }
  });
}

// Initialize edit and delete forms when DOM is loaded
document.addEventListener('DOMContentLoaded', function () {
  initializeEditForm();
  initializeDeleteButton();
});
