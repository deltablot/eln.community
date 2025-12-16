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

  // Initialize ROR autocomplete for edit page
  initializeRorAutocomplete('ror-search-input', 'ror-search-results', 'selected-rors', 'rors-hidden-input');
  
  // Initialize ROR autocomplete for upload page
  initializeRorAutocomplete('ror-search-input-upload', 'ror-search-results-upload', 'selected-rors-upload', 'rors-hidden-input-upload');
  
  // Load ROR names for record page
  loadRorNames();
  
  // Load ROR names for browse page
  loadRorNamesForBrowse();

  // Handle category select change for browse page
  const categorySelect = document.getElementById('category');
  const searchForm = document.getElementById('searchForm');
  if (categorySelect && searchForm) {
    categorySelect.addEventListener('change', function() {
      searchForm.submit();
    });
  }

  // Initialize pagination for browse page
  initializePagination();

  // Initialize browse page search and filter
  initializeBrowseSearch();

  // Initialize AG Grid for browse page
  initializeBrowseGrid();

  // Format relative timestamps
  formatRelativeTimes();
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
          <i class="bi bi-download"></i>
          Download HTML
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
    
    // Create a temporary link and trigger download instead of popup
    const link = document.createElement('a');
    link.href = url;
    link.download = `${entityId.replace(/[^a-zA-Z0-9]/g, '_')}.html`;
    link.style.display = 'none';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);

    // Clean up the blob URL after a short delay
    setTimeout(() => {
      URL.revokeObjectURL(url);
    }, 1000);
  } catch (error) {
    console.error('Error downloading HTML content:', error);
    alert('Error downloading HTML content: ' + error.message);
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
    const fileInput = document.getElementById('file-input');
    const hasFile = fileInput && fileInput.files && fileInput.files.length > 0;

    if (submitBtn) {
      submitBtn.disabled = true;
      submitBtn.textContent = hasFile ? 'Uploading new version...' : 'Updating...';
    }

    // Use FormData to support file uploads
    const formData = new FormData(this);

    try {
      const response = await fetch(this.action, {
        method: 'POST',
        body: formData // Send as multipart/form-data
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

// ROR Autocomplete functionality
let rorSearchTimeout = null;
const rorCache = new Map();

function initializeRorAutocomplete(inputId, resultsId, selectedId, hiddenInputId) {
  const searchInput = document.getElementById(inputId);
  const searchResults = document.getElementById(resultsId);
  const selectedRors = document.getElementById(selectedId);
  const hiddenInput = document.getElementById(hiddenInputId);

  if (!searchInput || !searchResults || !selectedRors || !hiddenInput) {
    return; // Not on a page with ROR autocomplete
  }

  // Load existing ROR names for edit page
  loadExistingRorNames(selectedId);

  // Handle search input
  searchInput.addEventListener('input', function(e) {
    const query = e.target.value.trim();
    
    if (query.length < 2) {
      searchResults.classList.add('d-none');
      searchResults.innerHTML = '';
      return;
    }

    // Debounce search
    clearTimeout(rorSearchTimeout);
    rorSearchTimeout = setTimeout(() => {
      searchRorOrganizations(query, resultsId, selectedId, hiddenInputId, inputId);
    }, 300);
  });

  // Handle clicks outside to close results
  document.addEventListener('click', function(e) {
    if (!searchInput.contains(e.target) && !searchResults.contains(e.target)) {
      searchResults.classList.add('d-none');
    }
  });

  // Handle remove button clicks
  selectedRors.addEventListener('click', function(e) {
    const badge = e.target.closest('.ror-badge');
    if (badge) {
      e.preventDefault();
      e.stopPropagation();
      badge.remove();
      updateHiddenInput(selectedId, hiddenInputId);
    }
  });
}

async function searchRorOrganizations(query, resultsId, selectedId, hiddenInputId, inputId) {
  const searchResults = document.getElementById(resultsId);
  
  try {
    searchResults.innerHTML = '<div class="list-group-item">Searching...</div>';
    searchResults.classList.remove('d-none');

    const response = await fetch(`/api/v1/ror/search?q=${encodeURIComponent(query)}`);
    
    if (!response.ok) {
      throw new Error('Search failed');
    }

    const organizations = await response.json();
    
    if (organizations.length === 0) {
      searchResults.innerHTML = '<div class="list-group-item">No results found</div>';
      return;
    }

    // Cache results
    organizations.forEach(org => {
      rorCache.set(org.id, org);
    });

    // Display results
    searchResults.innerHTML = '';
    organizations.forEach(org => {
      const item = document.createElement('button');
      item.type = 'button';
      item.className = 'list-group-item list-group-item-action';
      item.innerHTML = `
        <div class="d-flex w-100 justify-content-between">
          <h6 class="mb-1">${escapeHtml(org.name)}</h6>
          <small class="text-muted">${escapeHtml(org.id)}</small>
        </div>
        ${org.types && org.types.length > 0 ? `<small class="text-muted">${org.types.map(t => escapeHtml(t)).join(', ')}</small>` : ''}
      `;
      
      item.addEventListener('click', function() {
        addRorOrganization(org, selectedId, hiddenInputId);
        searchResults.classList.add('d-none');
        document.getElementById(inputId).value = '';
      });
      
      searchResults.appendChild(item);
    });
  } catch (error) {
    console.error('Error searching ROR organizations:', error);
    searchResults.innerHTML = '<div class="list-group-item text-danger">Error searching organizations</div>';
  }
}

function addRorOrganization(org, selectedId, hiddenInputId) {
  const selectedRors = document.getElementById(selectedId);
  
  // Check if already added
  const existing = selectedRors.querySelector(`[data-ror-id="${org.id}"]`);
  if (existing) {
    return;
  }

  // Create badge
  const badge = document.createElement('button');
  badge.type = 'button';
  badge.className = 'btn btn-primary me-2 mb-2 ror-badge';
  badge.setAttribute('data-ror-id', org.id);
  badge.innerHTML = `
    <span class="ror-name">${escapeHtml(org.name)}</span>
    <span class="badge bg-light text-dark ms-2">×</span>
  `;
  
  selectedRors.appendChild(badge);
  updateHiddenInput(selectedId, hiddenInputId);
}

function updateHiddenInput(selectedId, hiddenInputId) {
  const selectedRors = document.getElementById(selectedId);
  const hiddenInput = document.getElementById(hiddenInputId);
  
  const badges = selectedRors.querySelectorAll('.ror-badge');
  const rorIds = Array.from(badges).map(badge => badge.getAttribute('data-ror-id'));
  
  hiddenInput.value = rorIds.join(', ');
}

async function loadExistingRorNames(selectedId) {
  const selectedRors = document.getElementById(selectedId);
  if (!selectedRors) return;

  const badges = selectedRors.querySelectorAll('.ror-badge');
  if (badges.length === 0) return;

  const rorIds = Array.from(badges).map(badge => badge.getAttribute('data-ror-id'));
  
  try {
    const response = await fetch(`/api/v1/ror/organizations?ids=${rorIds.join(',')}`);
    if (!response.ok) {
      console.error('Failed to load ROR names');
      return;
    }

    const organizations = await response.json();
    
    // Update badges with names
    organizations.forEach(org => {
      rorCache.set(org.id, org);
      const badge = selectedRors.querySelector(`[data-ror-id="${org.id}"]`);
      if (badge) {
        const nameSpan = badge.querySelector('.ror-name');
        if (nameSpan) {
          nameSpan.textContent = org.name;
        }
      }
    });
  } catch (error) {
    console.error('Error loading ROR names:', error);
  }
}

async function loadRorNames() {
  const rorIdsElement = document.getElementById('ror-ids-data');
  if (!rorIdsElement) {
    return; // Not on a record page with ROR IDs
  }

  const loadingElement = document.getElementById('ror-organizations-loading');
  const displayElement = document.getElementById('ror-organizations');
  
  if (!loadingElement || !displayElement) {
    return;
  }

  try {
    const rorIds = JSON.parse(rorIdsElement.textContent);
    
    if (!rorIds || rorIds.length === 0) {
      loadingElement.classList.add('d-none');
      return;
    }

    const organizations = await fetchRorOrganizations(rorIds);
    
    // Build display HTML
    let html = '';
    organizations.forEach((org, index) => {
      if (index > 0) html += ', ';
      html += `<a href='https://ror.org/${escapeHtml(org.id)}' target='_blank' rel='noopener noreferrer'>${escapeHtml(org.name)}</a>`;
    });
    
    displayElement.innerHTML = html;
    loadingElement.classList.add('d-none');
    displayElement.classList.remove('d-none');
  } catch (error) {
    console.error('Error loading ROR names:', error);
    loadingElement.textContent = 'Error loading organization names';
  }
}

async function loadRorNamesForBrowse() {
  const rorElements = document.querySelectorAll('.ror-organizations[data-ror-ids]');
  if (rorElements.length === 0) {
    return; // Not on browse page or no ROR IDs
  }

  // Collect all unique ROR IDs from all records
  const allRorIds = new Set();
  rorElements.forEach(element => {
    const rorIds = element.getAttribute('data-ror-ids').split(',').filter(id => id.trim());
    rorIds.forEach(id => allRorIds.add(id.trim()));
  });

  if (allRorIds.size === 0) {
    return;
  }

  try {
    // Fetch all organizations in one batch
    const organizations = await fetchRorOrganizations(Array.from(allRorIds));
    
    // Create a map for quick lookup
    const orgMap = new Map();
    organizations.forEach(org => {
      orgMap.set(org.id, org);
    });

    // Update each record's ROR display
    rorElements.forEach(element => {
      const recordId = element.getAttribute('data-record-id');
      const rorIds = element.getAttribute('data-ror-ids').split(',').filter(id => id.trim());
      const loadingElement = document.querySelector(`.ror-organizations-loading[data-record-id="${recordId}"]`);
      
      if (rorIds.length === 0) {
        if (loadingElement) loadingElement.classList.add('ror-hidden');
        return;
      }

      // Build display HTML with filter links
      let html = '';
      rorIds.forEach((rorId, index) => {
        const org = orgMap.get(rorId.trim());
        if (org) {
          if (index > 0) html += ', ';
          html += `<a href='/browse?ror=${encodeURIComponent(org.id)}' class='ror-filter-link' title='Filter by ${escapeHtml(org.name)}'>${escapeHtml(org.name)}</a>`;
        }
      });
      
      if (html) {
        element.innerHTML = html;
        element.classList.remove('ror-hidden');
        element.classList.add('ror-inline');
      }
      
      if (loadingElement) {
        loadingElement.classList.add('ror-hidden');
      }
    });
  } catch (error) {
    console.error('Error loading ROR names for browse:', error);
    // Hide loading indicators on error
    document.querySelectorAll('.ror-organizations-loading').forEach(el => {
      el.textContent = 'Error loading';
    });
  }
}

// Fetch ROR organizations with caching
async function fetchRorOrganizations(rorIds) {
  if (!Array.isArray(rorIds) || rorIds.length === 0) {
    return [];
  }

  // Check cache first
  const uncachedIds = [];
  const cachedOrgs = [];
  
  rorIds.forEach(id => {
    if (rorCache.has(id)) {
      cachedOrgs.push(rorCache.get(id));
    } else {
      uncachedIds.push(id);
    }
  });

  // If all are cached, return immediately
  if (uncachedIds.length === 0) {
    return cachedOrgs;
  }

  // Fetch uncached IDs
  try {
    const response = await fetch(`/api/v1/ror/organizations?ids=${uncachedIds.join(',')}`);
    if (!response.ok) {
      throw new Error('Failed to load ROR organizations');
    }

    const organizations = await response.json();
    
    // Cache the results
    organizations.forEach(org => {
      rorCache.set(org.id, org);
    });

    // Return all organizations (cached + newly fetched)
    return [...cachedOrgs, ...organizations];
  } catch (error) {
    console.error('Error fetching ROR organizations:', error);
    // Return cached ones even if fetch fails
    return cachedOrgs;
  }
}

// Format timestamps as relative time (e.g., "2 weeks ago")
function formatRelativeTimes() {
  const timeElements = document.querySelectorAll('.relative-time');
  
  timeElements.forEach(element => {
    const card = element.closest('.record-card-date');
    if (!card) return;
    
    const timestamp = parseInt(card.getAttribute('data-timestamp'));
    if (!timestamp) return;
    
    const date = new Date(timestamp * 1000);
    const now = new Date();
    const diffMs = now - date;
    const diffSecs = Math.floor(diffMs / 1000);
    const diffMins = Math.floor(diffSecs / 60);
    const diffHours = Math.floor(diffMins / 60);
    const diffDays = Math.floor(diffHours / 24);
    const diffWeeks = Math.floor(diffDays / 7);
    const diffMonths = Math.floor(diffDays / 30);
    const diffYears = Math.floor(diffDays / 365);
    
    let relativeTime;
    
    if (diffSecs < 60) {
      relativeTime = 'just now';
    } else if (diffMins < 60) {
      relativeTime = diffMins === 1 ? '1 minute ago' : `${diffMins} minutes ago`;
    } else if (diffHours < 24) {
      relativeTime = diffHours === 1 ? '1 hour ago' : `${diffHours} hours ago`;
    } else if (diffDays < 7) {
      relativeTime = diffDays === 1 ? '1 day ago' : `${diffDays} days ago`;
    } else if (diffWeeks < 5) {
      relativeTime = diffWeeks === 1 ? '1 week ago' : `${diffWeeks} weeks ago`;
    } else if (diffMonths < 12) {
      relativeTime = diffMonths === 1 ? '1 month ago' : `${diffMonths} months ago`;
    } else {
      relativeTime = diffYears === 1 ? '1 year ago' : `${diffYears} years ago`;
    }
    
    element.textContent = relativeTime;
    element.title = date.toLocaleString();
  });
}

// Initialize pagination for browse page
function initializePagination() {
  // Handle pagination clicks
  document.querySelectorAll('.pagination .page-link[data-page]').forEach(link => {
    link.addEventListener('click', function(e) {
      e.preventDefault();
      const page = this.getAttribute('data-page');
      navigateToBrowse({ page: page });
    });
  });
  
  // Handle page size change
  const pageSizeSelect = document.getElementById('pageSizeSelect');
  if (pageSizeSelect) {
    pageSizeSelect.addEventListener('change', function() {
      navigateToBrowse({ pageSize: this.value, page: '1' });
    });
  }
}

// Navigate to browse page with parameters
function navigateToBrowse(params = {}) {
  const url = new URL('/browse', window.location.origin);
  
  // Get current values from inputs
  const searchInput = document.getElementById('searchInput');
  const rorInput = document.getElementById('rorInput');
  const pageInput = document.getElementById('pageInput');
  const pageSizeInput = document.getElementById('pageSizeInput');
  const selectedCategoryBadge = document.querySelector('#selected-categories-badges .category-badge');
  
  // Build query parameters
  const queryParams = {};
  
  // Search query
  const searchValue = params.q !== undefined ? params.q : (searchInput ? searchInput.value.trim() : '');
  if (searchValue) {
    queryParams.q = searchValue;
  }
  
  // Category
  const categoryId = params.category !== undefined ? params.category : (selectedCategoryBadge ? selectedCategoryBadge.getAttribute('data-category-id') : '');
  if (categoryId) {
    queryParams.category = categoryId;
  }
  
  // ROR
  const rorValue = params.ror !== undefined ? params.ror : (rorInput ? rorInput.value.trim() : '');
  if (rorValue) {
    queryParams.ror = rorValue;
  }
  
  // Page
  const pageValue = params.page !== undefined ? params.page : (pageInput ? pageInput.value : '1');
  if (pageValue && pageValue !== '1') {
    queryParams.page = pageValue;
  }
  
  // Page size
  const pageSizeValue = params.pageSize !== undefined ? params.pageSize : (pageSizeInput ? pageSizeInput.value : '10');
  if (pageSizeValue && pageSizeValue !== '10') {
    queryParams.pageSize = pageSizeValue;
  }
  
  // Build URL with query parameters
  Object.keys(queryParams).forEach(key => {
    url.searchParams.append(key, queryParams[key]);
  });
  
  // Navigate
  window.location.href = url.toString();
}

// Initialize browse page search and filter
function initializeBrowseSearch() {
  const searchButton = document.getElementById('searchButton');
  const searchInput = document.getElementById('searchInput');
  const rorInput = document.getElementById('rorInput');
  
  if (!searchButton || !searchInput) {
    return; // Not on browse page
  }
  
  // Handle search button click
  searchButton.addEventListener('click', function() {
    navigateToBrowse({ page: '1' });
  });
  
  // Handle Enter key in search input
  searchInput.addEventListener('keypress', function(e) {
    if (e.key === 'Enter') {
      e.preventDefault();
      navigateToBrowse({ page: '1' });
    }
  });
  
  // Handle Enter key in ROR input
  if (rorInput) {
    rorInput.addEventListener('keypress', function(e) {
      if (e.key === 'Enter') {
        e.preventDefault();
        navigateToBrowse({ page: '1' });
      }
    });
  }
}

// Category search functionality
function initializeCategorySearch() {
  // Search in browse page
  const browseSearch = document.getElementById('browse-category-search');
  if (browseSearch) {
    const browseTree = document.getElementById('browse-category-tree');
    browseSearch.addEventListener('input', function() {
      filterCategories(this.value, browseTree);
    });
  }

  // Search in new page dropdown
  const newSearch = document.getElementById('new-category-search');
  if (newSearch) {
    const newDropdown = document.getElementById('category-selector-dropdown');
    newSearch.addEventListener('input', function() {
      filterCategories(this.value, newDropdown);
    });
  }

  // Search in edit page dropdown
  const editSearch = document.getElementById('edit-category-search');
  if (editSearch) {
    const editDropdown = document.getElementById('category-selector-dropdown');
    editSearch.addEventListener('input', function() {
      filterCategories(this.value, editDropdown);
    });
  }
}

function filterCategories(searchTerm, container) {
  if (!container) return;

  const term = searchTerm.toLowerCase().trim();
  const allItems = container.querySelectorAll('.category-tree-item');
  const noResults = container.querySelector('.category-no-results');
  let visibleCount = 0;

  // Hide no results message if search is empty
  if (term.length === 0) {
    // Show all items, collapse all
    allItems.forEach(item => {
      item.classList.remove('search-hidden');
      const categoryItem = item.querySelector('.category-item');
      const children = item.querySelector(':scope > .category-children');
      const toggle = item.querySelector('.category-toggle');
      
      if (children) {
        children.classList.remove('expanded');
        if (toggle) toggle.classList.remove('expanded');
        if (categoryItem) categoryItem.classList.remove('expanded');
      }
    });
    if (noResults) noResults.style.display = 'none';
    return;
  }

  // Filter categories
  allItems.forEach(item => {
    const categoryName = item.querySelector('.category-name');
    if (!categoryName) return;

    const text = categoryName.textContent.toLowerCase();
    const matches = text.includes(term);

    if (matches) {
      // Show matching item
      item.classList.remove('search-hidden');
      item.classList.add('search-match');
      visibleCount++;

      // Show all parent items
      let parent = item.parentElement;
      while (parent) {
        if (parent.classList.contains('category-tree-item')) {
          parent.classList.remove('search-hidden');
          
          // Expand parent
          const parentChildren = parent.querySelector(':scope > .category-children');
          const parentToggle = parent.querySelector(':scope > .category-item > .category-toggle');
          const parentItem = parent.querySelector(':scope > .category-item');
          
          if (parentChildren) {
            parentChildren.classList.add('expanded');
            if (parentToggle) parentToggle.classList.add('expanded');
            if (parentItem) parentItem.classList.add('expanded');
          }
        }
        parent = parent.parentElement;
      }

      // Expand children if this item has any
      const children = item.querySelector(':scope > .category-children');
      const toggle = item.querySelector('.category-toggle');
      const categoryItem = item.querySelector('.category-item');
      
      if (children) {
        children.classList.add('expanded');
        if (toggle) toggle.classList.add('expanded');
        if (categoryItem) categoryItem.classList.add('expanded');
      }
    } else {
      // Hide non-matching items (will be shown if they're parents of matches)
      item.classList.add('search-hidden');
      item.classList.remove('search-match');
    }
  });

  // Show/hide no results message
  if (noResults) {
    noResults.style.display = visibleCount === 0 ? 'block' : 'none';
  }
}

// Initialize category multi-select
function initializeCategoryMultiselect() {
  const multiselectInput = document.getElementById('categoryMultiselectInput');
  const multiselectDropdown = document.getElementById('categoryMultiselectDropdown');
  const multiselectTags = document.getElementById('categoryMultiselectTags');
  const multiselectPlaceholder = document.getElementById('categoryMultiselectPlaceholder');
  
  if (!multiselectInput || !multiselectDropdown || !multiselectTags) {
    return; // Not on browse page
  }
  
  // Track selected categories
  const selectedCategories = new Set();
  
  // Initialize with existing selections
  const existingTags = multiselectTags.querySelectorAll('.category-tag');
  existingTags.forEach(tag => {
    const categoryId = tag.getAttribute('data-category-id');
    if (categoryId) {
      selectedCategories.add(categoryId);
    }
  });
  
  // Update placeholder visibility
  function updatePlaceholder() {
    if (selectedCategories.size > 0) {
      multiselectPlaceholder.classList.add('hidden');
    } else {
      multiselectPlaceholder.classList.remove('hidden');
    }
  }
  
  // Update hidden input with selected category IDs
  function updateHiddenCategoryInput() {
    const hiddenInput = document.getElementById('selected-categories-input');
    if (hiddenInput) {
      const categoryIds = Array.from(selectedCategories);
      hiddenInput.value = categoryIds.join(',');
    }
  }
  
  updatePlaceholder();
  updateHiddenCategoryInput();
  
  // Toggle dropdown
  multiselectInput.addEventListener('click', function(e) {
    e.stopPropagation();
    const isOpen = !multiselectDropdown.classList.contains('d-none');
    
    if (isOpen) {
      multiselectDropdown.classList.add('d-none');
      multiselectInput.classList.remove('open');
    } else {
      multiselectDropdown.classList.remove('d-none');
      multiselectInput.classList.add('open');
    }
  });
  
  // Close dropdown when clicking outside
  document.addEventListener('click', function(e) {
    if (!multiselectDropdown.contains(e.target) && !multiselectInput.contains(e.target)) {
      multiselectDropdown.classList.add('d-none');
      multiselectInput.classList.remove('open');
    }
  });
  
  // Handle checkbox changes
  const checkboxes = multiselectDropdown.querySelectorAll('.category-checkbox');
  checkboxes.forEach(checkbox => {
    checkbox.addEventListener('change', function(e) {
      e.stopPropagation();
      const categoryItem = this.closest('.category-item');
      const categoryId = categoryItem.getAttribute('data-category-id');
      const categoryName = categoryItem.getAttribute('data-category-name');
      
      if (this.checked) {
        // Add category
        selectedCategories.add(categoryId);
        addCategoryTag(categoryId, categoryName);
        categoryItem.classList.add('active');
      } else {
        // Remove category
        selectedCategories.delete(categoryId);
        removeCategoryTag(categoryId);
        categoryItem.classList.remove('active');
      }
      
      updatePlaceholder();
      updateHiddenCategoryInput();
    });
  });
  
  // Handle tag removal
  multiselectTags.addEventListener('click', function(e) {
    if (e.target.classList.contains('category-tag-remove')) {
      e.stopPropagation();
      const tag = e.target.closest('.category-tag');
      const categoryId = tag.getAttribute('data-category-id');
      
      // Remove from selected set
      selectedCategories.delete(categoryId);
      
      // Remove tag
      tag.remove();
      
      // Uncheck checkbox
      const checkbox = multiselectDropdown.querySelector(`.category-item[data-category-id="${categoryId}"] .category-checkbox`);
      if (checkbox) {
        checkbox.checked = false;
        checkbox.closest('.category-item').classList.remove('active');
      }
      
      updatePlaceholder();
      updateHiddenCategoryInput();
    }
  });
  
  // Add category tag
  function addCategoryTag(categoryId, categoryName) {
    // Check if already exists
    if (multiselectTags.querySelector(`[data-category-id="${categoryId}"]`)) {
      return;
    }
    
    const tag = document.createElement('span');
    tag.className = 'category-tag';
    tag.setAttribute('data-category-id', categoryId);
    tag.innerHTML = `
      <span class="category-tag-text">${escapeHtml(categoryName)}</span>
      <span class="category-tag-remove">×</span>
    `;
    
    multiselectTags.appendChild(tag);
  }
  
  // Remove category tag
  function removeCategoryTag(categoryId) {
    const tag = multiselectTags.querySelector(`[data-category-id="${categoryId}"]`);
    if (tag) {
      tag.remove();
    }
  }

  // Apply filters button
  const applyBtn = document.getElementById('categoryApplyBtn');
  if (applyBtn) {
    applyBtn.addEventListener('click', function() {
      const categoryIds = Array.from(selectedCategories);
      if (categoryIds.length > 0) {
        // Send multiple categories as comma-separated values
        navigateToBrowse({ category: categoryIds.join(','), page: '1' });
      } else {
        navigateToBrowse({ category: '', page: '1' });
      }
    });
  }
  
  // Clear all button
  const clearBtn = document.getElementById('categoryClearBtn');
  const clearBtnEdit = document.getElementById('categoryClearBtnEdit');
  
  if (clearBtn) {
    clearBtn.addEventListener('click', function(e) {
      e.stopPropagation();
      
      // Clear all selections
      selectedCategories.clear();
      
      // Remove all tags
      multiselectTags.innerHTML = '';
      
      // Uncheck all checkboxes
      checkboxes.forEach(checkbox => {
        checkbox.checked = false;
        checkbox.closest('.category-item').classList.remove('active');
      });
      
      updatePlaceholder();
      updateHiddenCategoryInput();
      
      // Only navigate if on browse page (has Apply button)
      const applyBtn = document.getElementById('categoryApplyBtn');
      if (applyBtn) {
        navigateToBrowse({ category: '', page: '1' });
      }
    });
  }
  
  if (clearBtnEdit) {
    clearBtnEdit.addEventListener('click', function(e) {
      e.stopPropagation();
      
      // Clear all selections
      selectedCategories.clear();
      
      // Remove all tags
      multiselectTags.innerHTML = '';
      
      // Uncheck all checkboxes
      checkboxes.forEach(checkbox => {
        checkbox.checked = false;
        checkbox.closest('.category-item').classList.remove('active');
      });
      
      updatePlaceholder();
      updateHiddenCategoryInput();
    });
  }
}

// Category Tree functionality
function initializeCategoryTree() {
  // Handle toggle clicks for expanding/collapsing - use event delegation for better compatibility
  document.addEventListener('click', function(e) {
    // Check if clicked element is a category toggle
    if (e.target.classList.contains('category-toggle') && !e.target.classList.contains('no-children')) {
      e.stopPropagation();
      e.preventDefault();
      
      const toggle = e.target;
      const categoryItem = toggle.closest('.category-item');
      const treeItem = toggle.closest('.category-tree-item');
      const children = treeItem ? treeItem.querySelector(':scope > .category-children') : null;
      
      if (children) {
        // Toggle expanded state
        const isExpanded = children.classList.contains('expanded');

        if (isExpanded) {
          // Collapse
          children.classList.remove('expanded');
          toggle.classList.remove('expanded');
          if (categoryItem) categoryItem.classList.remove('expanded');
        } else {
          // Expand
          children.classList.add('expanded');
          toggle.classList.add('expanded');
          if (categoryItem) categoryItem.classList.add('expanded');
        }
      }
    }
  });

  // Handle category selection for browse page (single select with badge UI)
  const browseCategoryItems = document.querySelectorAll('.category-tree-container .category-item');
  const selectedCategoriesBadges = document.getElementById('selected-categories-badges');

  browseCategoryItems.forEach(item => {
    item.addEventListener('click', function(e) {
      // Don't trigger if clicking on toggle
      if (e.target.classList.contains('category-toggle')) {
        return;
      }

      const categoryId = this.getAttribute('data-category-id');
      const categoryName = this.getAttribute('data-category-name');

      if (!categoryId || !categoryName) return;

      // Check if already selected
      const existingBadge = selectedCategoriesBadges.querySelector(`[data-category-id="${categoryId}"]`);
      if (existingBadge) {
        // Remove if already selected (clear filter)
        navigateToBrowse({ category: '', page: '1' });
      } else {
        // Navigate with selected category
        navigateToBrowse({ category: categoryId, page: '1' });
      }
    });
  });

  // Handle badge removal
  if (selectedCategoriesBadges) {
    selectedCategoriesBadges.addEventListener('click', function(e) {
      const badge = e.target.closest('.category-badge');
      if (badge) {
        // Navigate without category filter
        navigateToBrowse({ category: '', page: '1' });
      }
    });
  }

  // Handle category selector for forms (new/edit pages)
  const selectorDisplay = document.getElementById('category-selector-display');
  const selectorDropdown = document.getElementById('category-selector-dropdown');
  const selectedCategoryInput = document.getElementById('selected-category-input');
  const selectedCategoryText = document.getElementById('selected-category-text');

  if (selectorDisplay && selectorDropdown && selectedCategoryInput && selectedCategoryText) {
    // Toggle dropdown on click
    selectorDisplay.addEventListener('click', function(e) {
      e.stopPropagation();
      selectorDropdown.classList.toggle('show');
      selectorDisplay.classList.toggle('open');
    });

    // Close dropdown when clicking outside
    document.addEventListener('click', function(e) {
      if (!selectorDropdown.contains(e.target) && e.target !== selectorDisplay) {
        selectorDropdown.classList.remove('show');
        selectorDisplay.classList.remove('open');
      }
    });

    // Handle category selection in dropdown
    selectorDropdown.querySelectorAll('.category-selectable').forEach(item => {
      item.addEventListener('click', function(e) {
        // Don't select if clicking on toggle
        if (e.target.classList.contains('category-toggle')) {
          return;
        }

        const categoryId = this.getAttribute('data-category-id');
        const categoryName = this.getAttribute('data-category-name');

        // Update hidden input
        selectedCategoryInput.value = categoryId;

        // Update display text
        if (categoryName && categoryName !== 'None') {
          selectedCategoryText.textContent = categoryName;
          selectedCategoryText.classList.remove('text-muted');
        } else {
          selectedCategoryText.textContent = 'None';
          selectedCategoryText.classList.add('text-muted');
        }

        // Remove active class from all items
        selectorDropdown.querySelectorAll('.category-item').forEach(i => {
          i.classList.remove('active');
        });

        // Add active class to selected item
        this.classList.add('active');

        // Close dropdown
        selectorDropdown.classList.remove('show');
        selectorDisplay.classList.remove('open');
      });
    });
  }
}

// Initialize category tree when DOM is loaded
document.addEventListener('DOMContentLoaded', function() {
  initializeCategoryTree();
  initializeCategorySearch();
  initializeCategoryMultiselect();
});


// Version History functionality
function initializeVersionHistory() {
  const recordIdElement = document.getElementById('record-id-data');
  if (!recordIdElement) {
    return; // Not on a record page
  }

  const recordId = JSON.parse(recordIdElement.textContent);
  const versionCount = document.getElementById('version-count');
  const versionSelector = document.getElementById('version-selector');

  if (!versionSelector) {
    return; // Version selector not found
  }

  // Check if already initialized to prevent duplicates
  if (versionSelector.dataset.initialized === 'true') {
    return;
  }
  versionSelector.dataset.initialized = 'true';

  // Get current version from data attribute
  const currentVersion = versionSelector.getAttribute('data-current-version');

  // Fetch lightweight version list
  fetch(`/api/v1/records/${recordId}/versions`)
    .then(response => {
      if (!response.ok) throw new Error('Failed to fetch');
      return response.json();
    })
    .then(data => {
      const versions = data.versions || [];
      const totalVersions = versions.length + 1; // +1 for current
      
      if (versions.length > 0) {
        versionCount.textContent = `${totalVersions} total`;
        
        // Clear any existing options except "Current"
        while (versionSelector.options.length > 1) {
          versionSelector.remove(1);
        }
        
        // Populate dropdown with historical versions
        versions.forEach(version => {
          const option = document.createElement('option');
          option.value = version.version;
          const date = new Date(version.archived_at);
          option.textContent = `Version ${version.version} - ${version.name} (${date.toLocaleDateString()})`;
          
          // Select this option if it matches current version
          if (currentVersion === version.version.toString()) {
            option.selected = true;
          }
          
          versionSelector.appendChild(option);
        });
      } else {
        versionCount.textContent = '1 version';
      }
      
      // Handle dropdown selection - reload page with version query parameter
      versionSelector.addEventListener('change', (e) => {
        const selectedValue = e.target.value;
        if (selectedValue === 'current') {
          // Reload without version parameter
          window.location.href = `/record/${recordId}`;
        } else if (selectedValue) {
          // Reload with version parameter
          window.location.href = `/record/${recordId}?version=${selectedValue}`;
        }
      });
    })
    .catch(err => {
      console.error('Error loading version history:', err);
      versionCount.textContent = 'Error loading';
    });
}

// Initialize version history when DOM is loaded
document.addEventListener('DOMContentLoaded', function() {
  initializeVersionHistory();
});

// AG Grid initialization for browse page
function initializeBrowseGrid() {
  const gridDiv = document.getElementById('browseGrid');
  const dataElement = document.getElementById('browse-records-data');
  
  if (!gridDiv || !dataElement) {
    return; // Not on browse page with AG Grid
  }

  try {
    const data = JSON.parse(dataElement.textContent);
    const records = data.records || [];
    const pagination = data.pagination || {};
    const user = data.user;
    const isAdmin = data.isAdmin;

    // Helper to format relative time
    function formatRelativeTime(timestamp) {
      const now = Date.now();
      const date = new Date(timestamp * 1000);
      const diff = now - date.getTime();
      const seconds = Math.floor(diff / 1000);
      const minutes = Math.floor(seconds / 60);
      const hours = Math.floor(minutes / 60);
      const days = Math.floor(hours / 24);

      if (days > 30) {
        return date.toLocaleDateString();
      } else if (days > 0) {
        return `${days} day${days > 1 ? 's' : ''} ago`;
      } else if (hours > 0) {
        return `${hours} hour${hours > 1 ? 's' : ''} ago`;
      } else if (minutes > 0) {
        return `${minutes} minute${minutes > 1 ? 's' : ''} ago`;
      } else {
        return 'Just now';
      }
    }

    // Custom cell renderer for Name column with link
    function nameCellRenderer(params) {
      const link = document.createElement('a');
      link.href = `/record/${params.data.id}`;
      link.textContent = params.value;
      return link;
    }

    // Custom cell renderer for Categories column
    function categoriesCellRenderer(params) {
      const categories = params.value || [];
      if (categories.length === 0) {
        const span = document.createElement('span');
        span.className = 'text-muted';
        span.textContent = '-';
        return span;
      }
      
      const container = document.createElement('span');
      categories.forEach((cat, index) => {
        if (index > 0) {
          container.appendChild(document.createTextNode(', '));
        }
        const link = document.createElement('a');
        link.href = `/browse?category=${cat.id}`;
        link.textContent = cat.name;
        container.appendChild(link);
      });
      return container;
    }

    // Custom cell renderer for Organizations column
    function organizationsCellRenderer(params) {
      const rorIds = params.value || [];
      if (rorIds.length === 0) {
        const span = document.createElement('span');
        span.className = 'text-muted';
        span.textContent = '-';
        return span;
      }
      
      const container = document.createElement('span');
      container.className = 'ror-organizations';
      container.setAttribute('data-record-id', params.data.id);
      container.setAttribute('data-ror-ids', rorIds.join(','));
      container.textContent = 'Loading...';
      
      return container;
    }

    // Custom cell renderer for Created column
    function createdCellRenderer(params) {
      const div = document.createElement('div');
      div.className = 'record-card-date';
      const span = document.createElement('span');
      span.className = 'relative-time';
      span.textContent = formatRelativeTime(params.value);
      div.appendChild(span);
      return div;
    }

    // Custom cell renderer for Actions column
    function actionsCellRenderer(params) {
      const container = document.createElement('div');
      container.className = 'text-end';
      
      // View button
      const viewBtn = document.createElement('a');
      viewBtn.className = 'btn btn-sm btn-outline-primary me-1';
      viewBtn.href = `/record/${params.data.id}`;
      viewBtn.textContent = 'View';
      container.appendChild(viewBtn);
      
      // Download button
      const downloadBtn = document.createElement('a');
      downloadBtn.className = 'btn btn-sm btn-outline-secondary me-1';
      downloadBtn.href = `/api/v1/record/${params.data.id}.eln`;
      downloadBtn.textContent = 'Download';
      container.appendChild(downloadBtn);
      
      // Edit button (only for owner or admin)
      const canEdit = isAdmin || (user && user.orcid === params.data.uploaderOrcid);
      if (canEdit) {
        const editBtn = document.createElement('a');
        editBtn.className = 'btn btn-sm btn-outline-primary';
        editBtn.href = `/api/v1/record/${params.data.id}/edit`;
        editBtn.textContent = 'Edit';
        container.appendChild(editBtn);
      }
      
      return container;
    }

    // Column definitions
    const columnDefs = [
      { 
        field: 'name', 
        headerName: 'Name',
        cellRenderer: nameCellRenderer,
        flex: 2,
        minWidth: 150,
        filter: true,
        sortable: true
      },
      { 
        field: 'uploaderName', 
        headerName: 'Author',
        flex: 1,
        minWidth: 100,
        filter: true,
        sortable: true
      },
      { 
        field: 'categories', 
        headerName: 'Categories',
        cellRenderer: categoriesCellRenderer,
        flex: 1.5,
        minWidth: 120,
        sortable: false,
        filter: false
      },
      { 
        field: 'rorIds', 
        headerName: 'Organizations',
        cellRenderer: organizationsCellRenderer,
        flex: 1.5,
        minWidth: 120,
        sortable: false,
        filter: false
      },
      { 
        field: 'downloadCount', 
        headerName: 'Downloads',
        flex: 0.7,
        minWidth: 80,
        filter: true,
        sortable: true
      },
      { 
        field: 'createdAt', 
        headerName: 'Created',
        cellRenderer: createdCellRenderer,
        flex: 1,
        minWidth: 100,
        sortable: true,
        filter: false
      },
      { 
        headerName: 'Actions',
        cellRenderer: actionsCellRenderer,
        flex: 1.5,
        minWidth: 180,
        sortable: false,
        filter: false,
        suppressSizeToFit: true
      }
    ];

    // Grid options
    const gridOptions = {
      columnDefs: columnDefs,
      rowData: records,
      domLayout: 'autoHeight',
      suppressHorizontalScroll: false,
      defaultColDef: {
        resizable: true
      },
      pagination: true,
      paginationPageSize: pagination.pageSize || 10,
      paginationPageSizeSelector: [10, 20, 30, 50],
      suppressPaginationPanel: false,
      animateRows: true,
      rowHeight: 48,
      headerHeight: 40,
      onGridReady: function(params) {
        params.api.sizeColumnsToFit();
      },
      onFirstDataRendered: function(params) {
        params.api.sizeColumnsToFit();
      }
    };

    // Create the grid
    agGrid.createGrid(gridDiv, gridOptions);
    
    // Batch load all ROR names after grid is rendered
    setTimeout(() => batchLoadRorNamesForGrid(), 0);

  } catch (error) {
    console.error('Error initializing AG Grid:', error);
    gridDiv.innerHTML = '<div class="alert alert-danger">Error loading records table</div>';
  }
}

// Batch load ROR names for all cells in the grid
async function batchLoadRorNamesForGrid() {
  const containers = document.querySelectorAll('#browseGrid .ror-organizations[data-ror-ids]');
  if (containers.length === 0) return;
  
  // Collect all unique ROR IDs from all cells
  const allRorIds = new Set();
  containers.forEach(container => {
    const rorIds = container.getAttribute('data-ror-ids').split(',').filter(id => id.trim());
    rorIds.forEach(id => allRorIds.add(id.trim()));
  });
  
  if (allRorIds.size === 0) return;
  
  try {
    // Make a single request for all ROR IDs using the cached fetch function
    const organizations = await fetchRorOrganizations(Array.from(allRorIds));
    
    // Create a map for quick lookup
    const orgMap = new Map();
    organizations.forEach(org => {
      orgMap.set(org.id, org);
    });
    
    // Update each cell with the fetched data
    containers.forEach(container => {
      const rorIds = container.getAttribute('data-ror-ids').split(',').filter(id => id.trim());
      container.innerHTML = '';
      
      rorIds.forEach((rorId, index) => {
        const org = orgMap.get(rorId.trim());
        if (org) {
          if (index > 0) {
            container.appendChild(document.createTextNode(', '));
          }
          const link = document.createElement('a');
          link.href = `/browse?ror=${encodeURIComponent(org.id)}`;
          link.textContent = org.name;
          link.title = org.id;
          container.appendChild(link);
        }
      });
      
      // If no orgs were found, show the IDs
      if (container.innerHTML === '') {
        container.textContent = rorIds.join(', ');
      }
    });
  } catch (error) {
    console.error('Error batch loading ROR names:', error);
    // Show IDs on error
    containers.forEach(container => {
      const rorIds = container.getAttribute('data-ror-ids').split(',').filter(id => id.trim());
      container.textContent = rorIds.join(', ');
    });
  }
}
