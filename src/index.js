// Intentionally high limit to support long user descriptions and multilingual content
const DESCRIPTION_MAX_LENGTH = 10000;

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
          // Hide the form and show success message
          const formContainer = document.getElementById('uploadFormContainer');
          const successMessage = document.getElementById('uploadSuccessMessage');
          if (formContainer && successMessage) {
            formContainer.classList.add('d-none');
            successMessage.classList.remove('d-none');
            // Scroll to top to ensure message is visible
            window.scrollTo({ top: 0, behavior: 'smooth' });
          }
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
          // Re-enable submit button on error
          if (submitBtn) {
            submitBtn.disabled = false;
            spinner?.classList.add('d-none');
            if (btnText) btnText.textContent = 'Send';
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
        console.error('Upload error:', error);
        // Re-enable submit button on error
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
    categorySelect.addEventListener('change', function () {
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

  const descriptionTextarea = document.getElementById('description');
  const descriptionCount = document.getElementById('description-count');
  const descriptionMax = document.getElementById('description-max');

  if (!descriptionTextarea || !descriptionCount || !descriptionMax) return;

  descriptionTextarea.maxLength = DESCRIPTION_MAX_LENGTH;
  descriptionMax.textContent = DESCRIPTION_MAX_LENGTH;

  const updateDescriptionCount = () => {
      descriptionCount.textContent = Array.from(descriptionTextarea.value).length;
  };

  descriptionTextarea.addEventListener('input', updateDescriptionCount);
  updateDescriptionCount();
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
  // Sanitize HTML content client-side before rendering in sandbox
  const sanitizedHTML = sanitizeHTML(htmlContent);

  // Create a complete HTML document with safe styling
  const styledHTML = `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            font-size: 14px;
            line-height: 1.5;
            color: #333;
            padding: 16px;
            margin: 0;
        }
        table { border-collapse: collapse; width: 100%; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f5f5f5; }
        img { max-width: 100%; height: auto; }
        a { color: #0066cc; text-decoration: none; }
        a:hover { text-decoration: underline; }
        pre, code { background: #f5f5f5; padding: 2px 4px; border-radius: 3px; font-family: monospace; }
        pre { padding: 12px; overflow-x: auto; }
        blockquote { border-left: 3px solid #ddd; margin-left: 0; padding-left: 16px; color: #666; }
    </style>
</head>
<body>${sanitizedHTML}</body>
</html>`;

  // Escape for srcdoc attribute
  const escapedHTML = styledHTML
    .replace(/&/g, '&amp;')
    .replace(/"/g, '&quot;');

  const safeEntityId = escapeHtml(entityId || 'unknown');

  // Render inline sandboxed iframe - no popup needed
  return `
    <div class="html-content-preview">
      <div class="d-flex align-items-center gap-2 mb-2">
        <span class="badge bg-info">HTML Content</span>
        <small class="text-muted">${safeEntityId}</small>
      </div>
      <div class="user-content-container">
        <iframe
          sandbox=""
          srcdoc="${escapedHTML}"
          title="HTML content from ${safeEntityId}"
          loading="lazy"
          onload="this.style.height = this.contentWindow.document.body.scrollHeight + 32 + 'px'"
        ></iframe>
      </div>
    </div>
  `;
}

// Client-side HTML sanitization using DOMParser
function sanitizeHTML(html) {
  // Create a temporary DOM to parse the HTML
  const parser = new DOMParser();
  const doc = parser.parseFromString(html, 'text/html');

  // Remove dangerous elements
  const dangerousElements = [
    'script', 'style', 'link', 'meta', 'base',
    'iframe', 'frame', 'frameset', 'object', 'embed', 'applet',
    'form', 'input', 'button', 'select', 'textarea'
  ];

  dangerousElements.forEach(tag => {
    const elements = doc.querySelectorAll(tag);
    elements.forEach(el => el.remove());
  });

  // Remove dangerous attributes from all elements
  const allElements = doc.querySelectorAll('*');
  allElements.forEach(el => {
    // Remove event handlers
    const attrs = Array.from(el.attributes);
    attrs.forEach(attr => {
      const name = attr.name.toLowerCase();
      // Remove event handlers (on*)
      if (name.startsWith('on')) {
        el.removeAttribute(attr.name);
      }
      // Remove style attribute
      if (name === 'style') {
        el.removeAttribute(attr.name);
      }
      // Remove javascript: URLs
      if ((name === 'href' || name === 'src') && attr.value.toLowerCase().trim().startsWith('javascript:')) {
        el.removeAttribute(attr.name);
      }
    });
  });

  return doc.body.innerHTML;
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

  const graph = data['@graph'];

  // Types to exclude from display
  const excludedTypes = ['CreateAction', 'CreativeWork', 'PropertyValue', 'SoftwareApplication', 'Thing'];

  // Build a lookup map for resolving references (e.g., Person by ID)
  const entityMap = {};
  graph.forEach(entity => {
    if (entity && entity['@id']) {
      entityMap[entity['@id']] = entity;
    }
  });

  // Helper to resolve author name from Person entity
  function resolveAuthorName(authorRef) {
    if (!authorRef) return null;
    const authorId = typeof authorRef === 'object' ? authorRef['@id'] : authorRef;
    const person = entityMap[authorId];
    if (person && person['@type'] === 'Person') {
      const givenName = person.givenName || '';
      const familyName = person.familyName || '';
      const email = person.email || '';
      if (givenName || familyName) {
        return { name: `${givenName} ${familyName}`.trim(), email };
      }
    }
    return null;
  }

  // Helper to resolve category/about name from Thing entity
  function resolveCategoryName(aboutRef) {
    if (!aboutRef) return null;
    const aboutId = typeof aboutRef === 'object' ? aboutRef['@id'] : aboutRef;
    const thing = entityMap[aboutId];
    if (thing && thing.name) {
      return thing.name;
    }
    return null;
  }

  // Find all Dataset entities (excluding root "./")
  const datasets = graph.filter(entity => {
    if (!entity || typeof entity !== 'object') return false;
    if (entity['@id'] === './') return false; // Skip root dataset
    const types = Array.isArray(entity['@type']) ? entity['@type'] : [entity['@type']];
    return types.includes('Dataset');
  });

  // Find Comment entities
  const comments = graph.filter(entity => {
    if (!entity || typeof entity !== 'object') return false;
    const types = Array.isArray(entity['@type']) ? entity['@type'] : [entity['@type']];
    return types.includes('Comment');
  });

  // Find Organization entities
  const organizations = graph.filter(entity => {
    if (!entity || typeof entity !== 'object') return false;
    const types = Array.isArray(entity['@type']) ? entity['@type'] : [entity['@type']];
    return types.includes('Organization');
  });

  let html = '';

  // Render Datasets
  if (datasets.length > 0) {
    html += '<h5 class="mb-3">Datasets</h5>';
    datasets.forEach(dataset => {
      html += renderDatasetCard(dataset, resolveAuthorName, resolveCategoryName);
    });
  }

  // Render Comments
  if (comments.length > 0) {
    html += '<h5 class="mt-4 mb-3">Comments</h5>';
    comments.forEach(comment => {
      html += renderCommentCard(comment, resolveAuthorName);
    });
  }

  // Render Organizations
  if (organizations.length > 0) {
    html += '<h5 class="mt-4 mb-3">Organizations</h5>';
    organizations.forEach(org => {
      html += renderOrganizationCard(org);
    });
  }

  if (html === '') {
    html = '<p class="text-muted">No displayable metadata found</p>';
  }

  return html;
}

/**
 * Render a Dataset entity as a card
 */
function renderDatasetCard(dataset, resolveAuthorName, resolveCategoryName) {
  const name = dataset.name || 'Unnamed Dataset';
  const author = resolveAuthorName(dataset.author);
  const category = resolveCategoryName(dataset.about);
  const status = dataset.creativeWorkStatus;
  const dateCreated = dataset.dateCreated ? formatDisplayDate(dataset.dateCreated) : null;
  const keywords = dataset.keywords;
  const url = dataset.url;

  let html = '<div class="card mb-3">';
  html += '<div class="card-body">';

  // Name
  html += `<h6 class="card-title fw-semibold mb-2">${escapeHtml(name)}</h6>`;

  // Author
  if (author) {
    html += `<div class="mb-1"><i class="bi bi-person me-2 text-secondary"></i>`;
    html += `<span class="text-muted">Author:</span> <span>${escapeHtml(author.name)}</span>`;
    if (author.email) {
      html += ` <small class="text-muted">(${escapeHtml(author.email)})</small>`;
    }
    html += '</div>';
  }

  // Category
  if (category) {
    html += `<div class="mb-1"><i class="bi bi-folder me-2 text-secondary"></i>`;
    html += `<span class="text-muted">Category:</span> <span class="badge bg-secondary">${escapeHtml(category)}</span></div>`;
  }

  // Status
  if (status) {
    const statusClass = status.toLowerCase().includes('success') ? 'bg-success' :
      status.toLowerCase().includes('fail') ? 'bg-danger' : 'bg-info';
    html += `<div class="mb-1"><i class="bi bi-flag me-2 text-secondary"></i>`;
    html += `<span class="text-muted">Status:</span> <span class="badge ${statusClass}">${escapeHtml(status)}</span></div>`;
  }

  // Date Created
  if (dateCreated) {
    html += `<div class="mb-1"><i class="bi bi-calendar me-2 text-secondary"></i>`;
    html += `<span class="text-muted">Created:</span> <span>${escapeHtml(dateCreated)}</span></div>`;
  }

  // Keywords/Tags
  if (keywords) {
    const tags = typeof keywords === 'string' ? keywords.split(',').map(t => t.trim()) : keywords;
    html += `<div class="mb-1"><i class="bi bi-tags me-2 text-secondary"></i>`;
    html += `<span class="text-muted">Tags:</span> `;
    tags.forEach(tag => {
      if (tag) {
        html += `<span class="badge bg-primary me-1">${escapeHtml(tag)}</span>`;
      }
    });
    html += '</div>';
  }

  // URL
  if (url) {
    html += `<div class="mb-1"><i class="bi bi-link me-2 text-secondary"></i>`;
    html += `<span class="text-muted">URL:</span> <a href="${escapeHtml(url)}" target="_blank" rel="noopener noreferrer" class="text-decoration-none">${escapeHtml(url)}</a></div>`;
  }

  html += '</div></div>';
  return html;
}

/**
 * Render a Comment entity as a card
 */
function renderCommentCard(comment, resolveAuthorName) {
  const author = resolveAuthorName(comment.author);
  const text = comment.text || '';
  const dateCreated = comment.dateCreated ? formatDisplayDate(comment.dateCreated) : null;

  let html = '<div class="card mb-2 border-start border-primary border-3">';
  html += '<div class="card-body py-2">';

  // Comment text
  html += `<p class="mb-2">${escapeHtml(text)}</p>`;

  // Author and date
  html += '<div class="small text-muted">';
  if (author) {
    html += `<i class="bi bi-person me-1"></i>${escapeHtml(author.name)}`;
  }
  if (dateCreated) {
    html += ` <i class="bi bi-clock ms-2 me-1"></i>${escapeHtml(dateCreated)}`;
  }
  html += '</div>';

  html += '</div></div>';
  return html;
}

/**
 * Render an Organization entity as a card
 */
function renderOrganizationCard(org) {
  const name = org.name || 'Unnamed Organization';
  const url = org.url;
  const slogan = org.slogan;

  let html = '<div class="card mb-2">';
  html += '<div class="card-body py-2">';

  // Name
  html += `<h6 class="card-title fw-semibold mb-1">${escapeHtml(name)}</h6>`;

  // Slogan
  if (slogan) {
    html += `<p class="small text-muted mb-1">${escapeHtml(slogan)}</p>`;
  }

  // URL
  if (url) {
    html += `<a href="${escapeHtml(url)}" target="_blank" rel="noopener noreferrer" class="small text-decoration-none">${escapeHtml(url)}</a>`;
  }

  html += '</div></div>';
  return html;
}

/**
 * Format a date string for display
 */
function formatDisplayDate(dateStr) {
  try {
    const date = new Date(dateStr);
    return date.toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    });
  } catch (e) {
    return dateStr;
  }
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
  contentDiv.addEventListener('click', function (e) {
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

    // Use new structured rendering with RecordExtractor
    renderStructuredRecordView(roCrateData);

    // Also render the traditional RO-Crate view in the content div
    contentDiv.innerHTML = renderRoCrate(roCrateData);
  } catch (error) {
    console.error('Error processing RO-Crate data:', error);
    const errorMessage = error instanceof Error ? error.message : String(error);
    contentDiv.innerHTML =
      '<div class="alert alert-danger">Error processing RO-Crate metadata: ' + escapeHtml(errorMessage) +
      '<br><small>Check browser console for more details</small></div>';
  }
}

/**
 * Render the structured record view using the RecordExtractor module
 * Populates the Common Info, Main Text, Extra Fields, and Other Metadata containers
 *
 * @param {Object} roCrateData - The parsed RO-Crate JSON data
 */
function renderStructuredRecordView(roCrateData) {
  // Check if RecordExtractor is available
  if (typeof window.RecordExtractor === 'undefined') {
    console.warn('RecordExtractor module not loaded, skipping structured view');
    return;
  }

  const {
    extractRecordData,
    renderCommonInfoBlock,
    renderMainTextBlock,
    renderExtraFieldsBlock,
    renderCustomFields,
    renderSteps,
  } = window.RecordExtractor;

  // Get fallback data from Record model
  const fallbackData = getFallbackRecordData();

  // Extract data from RO-Crate
  const extractedData = extractRecordData(roCrateData);

  // Apply fallbacks for missing data
  applyFallbackData(extractedData, fallbackData);

  // Render Common Info Block
  const commonInfoContainer = document.getElementById('common-info-container');
  if (commonInfoContainer && extractedData.commonInfo) {
    // Only update if we have meaningful data from RO-Crate
    const hasRoCrateData = extractedData.commonInfo.owner ||
      extractedData.commonInfo.team ||
      extractedData.commonInfo.tags.length > 0 ||
      extractedData.commonInfo.startDate;

    if (hasRoCrateData) {
      commonInfoContainer.innerHTML = renderCommonInfoBlock(extractedData.commonInfo);
    }
    // Otherwise keep the server-rendered fallback content
  }

  // Render Main Text Block
  const mainTextContainer = document.getElementById('main-text-container');
  if (mainTextContainer) {
    const hasMainText = extractedData.mainText.introduction;

    if (hasMainText) {
      mainTextContainer.innerHTML = renderMainTextBlock(extractedData.mainText);
    } else {
      // Hide the container if no main text content
      mainTextContainer.style.display = 'none';
    }
  }

  // Render Extra Fields Block
  const extraFieldsContainer = document.getElementById('extra-fields-container');
  if (extraFieldsContainer) {
    const hasExtraFields = extractedData.extraFields.attachedFiles.length > 0 ||
      extractedData.extraFields.experimentLinks.length > 0 ||
      extractedData.extraFields.resourceLinks.length > 0 ||
      extractedData.extraFields.compounds.length > 0 ||
      extractedData.extraFields.storage.length > 0;

    if (hasExtraFields) {
      extraFieldsContainer.innerHTML = renderExtraFieldsBlock(extractedData.extraFields);
    } else {
      // Hide the container if no extra fields
      extraFieldsContainer.style.display = 'none';
    }
  }

  // Render Custom Fields
  const customFieldContainer = document.getElementById('custom-field-container');
  if (customFieldContainer) {
      customFieldContainer.innerHTML = renderCustomFields(extractedData.extraFields.customFields);
    } else {
      // Hide the container if no custom field
      customFieldContainer.style.display = 'none';
    }

  // Render Steps
  const stepsContainer = document.getElementById('steps-container');
  if (stepsContainer) {
      stepsContainer.innerHTML = renderSteps(extractedData.extraFields.steps);
    } else {
      // Hide the container if no steps
      stepsContainer.style.display = 'none';
    }
}

/**
 * Get fallback data from the Record model (server-rendered data)
 * Used when RO-Crate metadata doesn't contain certain fields
 *
 * @returns {Object} - Fallback data object
 */
function getFallbackRecordData() {
  const fallbackElement = document.getElementById('record-fallback-data');
  if (!fallbackElement) {
    return {
      uploaderName: null,
      uploaderOrcid: null,
      createdAt: null,
      categories: []
    };
  }

  try {
    return JSON.parse(fallbackElement.textContent);
  } catch (e) {
    console.warn('Failed to parse fallback record data:', e);
    return {
      uploaderName: null,
      uploaderOrcid: null,
      createdAt: null,
      categories: []
    };
  }
}

/**
 * Apply fallback data to extracted data when RO-Crate fields are missing
 *
 * @param {Object} extractedData - The extracted data from RO-Crate
 * @param {Object} fallbackData - The fallback data from Record model
 */
function applyFallbackData(extractedData, fallbackData) {
  if (!extractedData || !fallbackData) return;

  // Fallback for owner: use uploader_name when no author in RO-Crate
  if (!extractedData.commonInfo.owner && fallbackData.uploaderName) {
    extractedData.commonInfo.owner = {
      name: fallbackData.uploaderName,
      orcid: fallbackData.uploaderOrcid || undefined
    };
  }

  // Fallback for tags: use categories when no keywords in RO-Crate
  if (extractedData.commonInfo.tags.length === 0 && fallbackData.categories && fallbackData.categories.length > 0) {
    extractedData.commonInfo.tags = fallbackData.categories.map(cat => cat.Name || cat.name);
  }

  // Fallback for start date: use created_at when no dateCreated in RO-Crate
  if (!extractedData.commonInfo.startDate && fallbackData.createdAt) {
    // Convert Go time format to ISO string if needed
    const createdAt = fallbackData.createdAt;
    if (typeof createdAt === 'string') {
      extractedData.commonInfo.startDate = createdAt;
    } else if (createdAt && createdAt.Time) {
      // Handle Go time.Time JSON format
      extractedData.commonInfo.startDate = createdAt.Time;
    }
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
        const formContainer = document.getElementById('editFormContainer');

        if (hasFile) {
          // New version uploaded - show moderation message
          const successMessage = document.getElementById('uploadSuccessMessage');
          if (formContainer && successMessage) {
            formContainer.classList.add('d-none');
            successMessage.classList.remove('d-none');
            window.scrollTo({ top: 0, behavior: 'smooth' });
          }
        } else {
          // Metadata update only - show simple success message
          const successMessage = document.getElementById('updateSuccessMessage');
          if (formContainer && successMessage) {
            formContainer.classList.add('d-none');
            successMessage.classList.remove('d-none');
            window.scrollTo({ top: 0, behavior: 'smooth' });
          }
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

// Handle license checkbox for edit page
function initializeLicenseCheckbox() {
  const fileInput = document.getElementById('file-input');
  const licenseContainer = document.getElementById('licenseAgreementContainer');
  const licenseCheckbox = document.getElementById('licenseAgreement');
  const editForm = document.querySelector('form.edit-record-form');

  if (!fileInput || !licenseContainer || !licenseCheckbox || !editForm) {
    return; // Not on edit page or elements not found
  }

  // Show/hide license checkbox based on file selection
  fileInput.addEventListener('change', function () {
    if (this.files && this.files.length > 0) {
      licenseContainer.style.display = 'block';
      licenseCheckbox.required = true;
      licenseCheckbox.checked = false;
    } else {
      licenseContainer.style.display = 'none';
      licenseCheckbox.required = false;
      licenseCheckbox.checked = false;
    }
  });

  // Validate license checkbox on form submit
  editForm.addEventListener('submit', function (e) {
    if (fileInput.files && fileInput.files.length > 0 && !licenseCheckbox.checked) {
      e.preventDefault();
      e.stopPropagation();

      // Show error toast
      const errorToast = document.getElementById('errorToast');
      const errorToastBody = document.getElementById('errorToastBody');
      if (errorToast && errorToastBody) {
        errorToastBody.textContent = 'Please agree to the license terms to upload a new version.';
        const toast = new bootstrap.Toast(errorToast);
        toast.show();
      }

      // Focus on the checkbox
      licenseCheckbox.focus();
      return false;
    }
  }, true); // Use capture phase to run before other submit handlers
}

// Handle archive button click
function initializeArchiveButton() {
  const archiveBtn = document.getElementById('archiveBtn');
  const archiveForm = document.querySelector('form.archive-record-form');
  const archiveModal = document.getElementById('archiveConfirmModal');
  const confirmArchiveBtn = document.getElementById('confirmArchiveBtn');
  const archiveReasonInput = document.getElementById('archiveReason');
  const archiveReasonHidden = document.getElementById('archiveReasonHidden');

  if (!archiveBtn || !archiveForm || !archiveModal || !confirmArchiveBtn) {
    return; // Not on edit page
  }

  // Enable/disable confirm button based on reason textarea
  if (archiveReasonInput) {
    archiveReasonInput.addEventListener('input', function () {
      confirmArchiveBtn.disabled = this.value.trim().length === 0;
    });
  }

  // Show modal when archive button is clicked
  archiveBtn.addEventListener('click', function (e) {
    e.preventDefault();
    const modal = new bootstrap.Modal(archiveModal);
    modal.show();
  });

  // Handle actual archive when confirmed
  confirmArchiveBtn.addEventListener('click', async function (e) {
    e.preventDefault();

    // Copy reason to hidden form field
    if (archiveReasonInput && archiveReasonHidden) {
      archiveReasonHidden.value = archiveReasonInput.value.trim();
    }

    const modal = bootstrap.Modal.getInstance(archiveModal);
    const originalText = confirmArchiveBtn.textContent;
    confirmArchiveBtn.disabled = true;
    confirmArchiveBtn.textContent = 'Archiving...';

    // Convert form data to URL-encoded format
    const formData = new FormData(archiveForm);
    const urlEncodedData = new URLSearchParams(formData).toString();

    // Extract the record ID from the form action URL
    const recordId = archiveForm.action.split('/api/v1/record/')[1];

    try {
      const response = await fetch(archiveForm.action, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/x-www-form-urlencoded',
        },
        body: urlEncodedData
      });

      if (response.ok) {
        // Hide modal
        modal.hide();

        // Redirect to record page
        window.location.href = '/record/' + recordId;
      } else {
        // Hide modal
        modal.hide();

        // Show error toast
        const errorText = await response.text();
        const errorToast = document.getElementById('errorToast');
        const errorToastBody = document.getElementById('errorToastBody');
        if (errorToast && errorToastBody) {
          errorToastBody.textContent = errorText || 'Archive failed. Please try again.';
          const toast = new bootstrap.Toast(errorToast);
          toast.show();
        }
        console.error('Archive failed:', errorText);

        // Reset button state
        confirmArchiveBtn.disabled = false;
        confirmArchiveBtn.textContent = originalText;
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
      console.error('Archive error:', error);

      // Reset button state
      confirmArchiveBtn.disabled = false;
      confirmArchiveBtn.textContent = originalText;
    }
  });
}

// Initialize edit and delete forms when DOM is loaded
document.addEventListener('DOMContentLoaded', function () {
  initializeEditForm();
  initializeArchiveButton();
  initializeLicenseCheckbox();
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
  searchInput.addEventListener('input', function (e) {
    const query = e.target.value.trim();

    if (query.length < 1) {
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
  document.addEventListener('click', function (e) {
    if (!searchInput.contains(e.target) && !searchResults.contains(e.target)) {
      searchResults.classList.add('d-none');
    }
  });

  // Handle remove button clicks
  selectedRors.addEventListener('click', function (e) {
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
    // Display results
    searchResults.innerHTML = '';
    organizations.forEach(org => {
      const item = document.createElement('button');
      item.type = 'button';
      item.className = 'list-group-item list-group-item-action';

      // Prepare country badge
      let countryHtml = '';
      if (org.country && org.country.country_name) {
        countryHtml = `<span class="badge bg-light text-dark border me-2">${escapeHtml(org.country.country_name)}</span>`;
      }

      // Prepare website link
      let websiteHtml = '';
      if (org.links && org.links.length > 0) {
        const url = org.links[0];
        if (url && (url.startsWith('http') || url.startsWith('https'))) {
          websiteHtml = `<a href="${escapeHtml(url)}" target="_blank" class="text-decoration-none small me-2" onclick="event.stopPropagation()"><span class="bi bi-globe"></span> Website</a>`;
        }
      }

      // Prepare aliases
      let aliasesHtml = '';
      if (org.aliases && org.aliases.length > 0) {
        const displayedAliases = org.aliases.slice(0, 3).join(', ');
        const moreCount = org.aliases.length - 3;
        const moreText = moreCount > 0 ? ` (+${moreCount} more)` : '';
        aliasesHtml = `<div class="small text-muted text-truncate mt-1" title="${escapeHtml(org.aliases.join(', '))}"><em>AKA: ${escapeHtml(displayedAliases)}${moreText}</em></div>`;
      }

      item.innerHTML = `
        <div class="d-flex w-100 justify-content-between">
          <h6 class="mb-1">${escapeHtml(org.name)}</h6>
          <small class="text-muted">${escapeHtml(org.id)}</small>
        </div>
        <div class="mb-1 d-flex align-items-center flex-wrap">
            ${org.types && org.types.length > 0 ? `<small class="text-muted me-2">${org.types.map(t => escapeHtml(t)).join(', ')}</small>` : ''}
            ${countryHtml}
            ${websiteHtml}
        </div>
        ${aliasesHtml}
      `;

      item.addEventListener('click', function () {
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

  let countryText = '';
  if (org.country && org.country.country_name) {
    countryText = ` <small class="opacity-75">(${escapeHtml(org.country.country_name)})</small>`;
  }

  badge.innerHTML = `
    <span class="ror-name">${escapeHtml(org.name)}</span>${countryText}
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

          if (org.country && org.country.country_name) {
            const countrySpan = document.createElement('small');
            countrySpan.className = 'opacity-75 ms-1';
            countrySpan.textContent = `(${org.country.country_name})`;
            nameSpan.after(countrySpan);
          }
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
      if (index > 0) html += '<br>';

      let countryText = '';
      if (org.country && org.country.country_name) {
        countryText = ` <span class="text-muted small">(${escapeHtml(org.country.country_name)})</span>`;
      }

      html += `<a href='https://ror.org/${escapeHtml(org.id)}' target='_blank' rel='noopener noreferrer'>${escapeHtml(org.name)}</a>${countryText}`;
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

          let countryText = '';
          if (org.country && org.country.country_name) {
            countryText = ` <span class="text-muted small">(${escapeHtml(org.country.country_name)})</span>`;
          }

          html += `<span class="ror-item"><a href='/browse?ror=${encodeURIComponent(org.id)}' class='ror-filter-link' title='Filter by ${escapeHtml(org.name)}'>${escapeHtml(org.name)}</a>${countryText}</span>`;
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

// Shared helper to format relative time with configurable options
function formatRelative(timestamp, options = {}) {
  const {
    capitalize = false,
    includeWeeks = true,
    includeMonths = true,
    includeYears = true,
    fallbackToDate = false,
    dateFallbackThreshold = 30
  } = options;

  const now = Date.now();
  const date = new Date(timestamp * 1000);
  const diff = now - date.getTime();
  const seconds = Math.floor(diff / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);
  const weeks = Math.floor(days / 7);
  const months = Math.floor(days / 30);
  const years = Math.floor(days / 365);

  let relativeTime;

  // Check if we should fall back to date string
  if (fallbackToDate && days > dateFallbackThreshold) {
    return date.toLocaleDateString();
  }

  // Determine the appropriate time unit
  if (seconds < 60) {
    relativeTime = capitalize ? 'Just now' : 'just now';
  } else if (minutes < 60) {
    relativeTime = minutes === 1 ? '1 minute ago' : `${minutes} minutes ago`;
  } else if (hours < 24) {
    relativeTime = hours === 1 ? '1 hour ago' : `${hours} hours ago`;
  } else if (days < 7) {
    relativeTime = days === 1 ? '1 day ago' : `${days} days ago`;
  } else if (includeWeeks && weeks < 5) {
    relativeTime = weeks === 1 ? '1 week ago' : `${weeks} weeks ago`;
  } else if (includeMonths && months < 12) {
    relativeTime = months === 1 ? '1 month ago' : `${months} months ago`;
  } else if (includeYears) {
    relativeTime = years === 1 ? '1 year ago' : `${years} years ago`;
  } else {
    // Fallback for when weeks/months/years are disabled
    relativeTime = days === 1 ? '1 day ago' : `${days} days ago`;
  }

  return relativeTime;
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
    const relativeTime = formatRelative(timestamp, {
      capitalize: false,
      includeWeeks: true,
      includeMonths: true,
      includeYears: true
    });

    element.textContent = relativeTime;
    element.title = date.toLocaleString();
  });
}

// Initialize pagination for browse page
function initializePagination() {
  // Handle pagination clicks
  document.querySelectorAll('.pagination .page-link[data-page]').forEach(link => {
    link.addEventListener('click', function (e) {
      e.preventDefault();
      const page = this.getAttribute('data-page');
      navigateToBrowse({ page: page });
    });
  });

  // Handle page size change
  const pageSizeSelect = document.getElementById('pageSizeSelect');
  if (pageSizeSelect) {
    pageSizeSelect.addEventListener('change', function () {
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
  searchButton.addEventListener('click', function () {
    navigateToBrowse({ page: '1' });
  });

  // Handle Enter key in search input
  searchInput.addEventListener('keypress', function (e) {
    if (e.key === 'Enter') {
      e.preventDefault();
      navigateToBrowse({ page: '1' });
    }
  });

  // Handle Enter key in ROR input
  if (rorInput) {
    rorInput.addEventListener('keypress', function (e) {
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
    browseSearch.addEventListener('input', function () {
      filterCategories(this.value, browseTree);
    });
  }

  // Search in new page dropdown
  const newSearch = document.getElementById('new-category-search');
  if (newSearch) {
    const newDropdown = document.getElementById('category-selector-dropdown');
    newSearch.addEventListener('input', function () {
      filterCategories(this.value, newDropdown);
    });
  }

  // Search in edit page dropdown
  const editSearch = document.getElementById('edit-category-search');
  if (editSearch) {
    const editDropdown = document.getElementById('category-selector-dropdown');
    editSearch.addEventListener('input', function () {
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
  multiselectInput.addEventListener('click', function (e) {
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
  document.addEventListener('click', function (e) {
    if (!multiselectDropdown.contains(e.target) && !multiselectInput.contains(e.target)) {
      multiselectDropdown.classList.add('d-none');
      multiselectInput.classList.remove('open');
    }
  });

  // Handle checkbox changes
  const checkboxes = multiselectDropdown.querySelectorAll('.category-checkbox');
  checkboxes.forEach(checkbox => {
    checkbox.addEventListener('change', function (e) {
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
  multiselectTags.addEventListener('click', function (e) {
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
    applyBtn.addEventListener('click', function () {
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
    clearBtn.addEventListener('click', function (e) {
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
    clearBtnEdit.addEventListener('click', function (e) {
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
  document.addEventListener('click', function (e) {
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
    item.addEventListener('click', function (e) {
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
    selectedCategoriesBadges.addEventListener('click', function (e) {
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
    selectorDisplay.addEventListener('click', function (e) {
      e.stopPropagation();
      selectorDropdown.classList.toggle('show');
      selectorDisplay.classList.toggle('open');
    });

    // Close dropdown when clicking outside
    document.addEventListener('click', function (e) {
      if (!selectorDropdown.contains(e.target) && e.target !== selectorDisplay) {
        selectorDropdown.classList.remove('show');
        selectorDisplay.classList.remove('open');
      }
    });

    // Handle category selection in dropdown
    selectorDropdown.querySelectorAll('.category-selectable').forEach(item => {
      item.addEventListener('click', function (e) {
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
document.addEventListener('DOMContentLoaded', function () {
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
      // Count only the archived versions (don't add +1 for current)
      const totalVersions = versions.length;

      if (versions.length > 0) {
        versionCount.textContent = `${totalVersions} versions`;

        // Clear any existing options except "Current"
        while (versionSelector.options.length > 1) {
          versionSelector.remove(1);
        }

        // Populate dropdown with historical versions
        versions.forEach(version => {
          const option = document.createElement('option');
          option.value = version.version;
          const date = new Date(version.archived_at);
          // Format date with time: "Jan 02, 2006 15:04"
          const formattedDate = date.toLocaleDateString('en-US', {
            month: 'short',
            day: '2-digit',
            year: 'numeric'
          }) + ' ' + date.toLocaleTimeString('en-US', {
            hour: '2-digit',
            minute: '2-digit',
            hour12: false
          });

          // Add moderation status indicator
          let statusText = '';
          if (version.moderation_status === 'pending') {
            statusText = ' [Pending Moderation]';
          } else if (version.moderation_status === 'rejected') {
            statusText = ' [Rejected]';
          } else if (version.moderation_status === 'flagged') {
            statusText = ' [Flagged]';
          }

          option.textContent = `Version ${version.version} - ${version.name} (${formattedDate})${statusText}`;

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

  // Handle "Get permalink" button
  const permalinkBtn = document.getElementById('get-permalink-btn');
  if (permalinkBtn) {
    permalinkBtn.addEventListener('click', () => {
      const selectedVersion = versionSelector.value;
      let permalinkUrl;

      if (selectedVersion === 'current') {
        // For current version, we need to get the actual version number
        // The current version is the latest archived version + 1, or 1 if no versions exist
        fetch(`/api/v1/records/${recordId}/versions`)
          .then(response => response.json())
          .then(data => {
            const versions = data.versions || [];
            const currentVersionNumber = versions.length > 0 ? Math.max(...versions.map(v => v.version)) + 1 : 1;
            permalinkUrl = `${window.location.origin}/record/${recordId}?version=${currentVersionNumber}`;
            copyPermalinkToClipboard(permalinkUrl);
          })
          .catch(err => {
            console.error('Error getting current version:', err);
            alert('Failed to generate permalink. Please try again.');
          });
      } else {
        // For historical versions, use the selected version number
        permalinkUrl = `${window.location.origin}/record/${recordId}?version=${selectedVersion}`;
        copyPermalinkToClipboard(permalinkUrl);
      }
    });
  }

  function copyPermalinkToClipboard(url) {
    navigator.clipboard.writeText(url).then(() => {
      // Show success feedback
      const originalText = permalinkBtn.innerHTML;
      permalinkBtn.innerHTML = '<i class="bi bi-check-lg"></i> Copied!';
      permalinkBtn.classList.remove('btn-outline-primary');
      permalinkBtn.classList.add('btn-success');

      setTimeout(() => {
        permalinkBtn.innerHTML = originalText;
        permalinkBtn.classList.remove('btn-success');
        permalinkBtn.classList.add('btn-outline-primary');
      }, 2000);
    }).catch(err => {
      console.error('Failed to copy permalink:', err);
      // Fallback: show the URL in a prompt
      prompt('Copy this permalink:', url);
    });
  }
}

// Initialize version history when DOM is loaded
document.addEventListener('DOMContentLoaded', function () {
  initializeVersionHistory();
});

// AG Grid initialization for browse page
function initializeBrowseGrid() {
  const gridDiv = document.getElementById('browseGrid');

  if (!gridDiv) {
    return; // Not on browse page with AG Grid
  }

  // Get user/admin info from data attributes on the grid div
  const user = gridDiv.dataset.userOrcid ? { orcid: gridDiv.dataset.userOrcid } : null;
  const isAdmin = gridDiv.dataset.isAdmin === 'true';
  const styleNonce = gridDiv.dataset.styleNonce || undefined;

  // Helper to format relative time (uses shared formatRelative)
  function formatRelativeTime(timestamp) {
    return formatRelative(timestamp, {
      capitalize: true,
      includeWeeks: false,
      includeMonths: false,
      includeYears: false,
      fallbackToDate: true,
      dateFallbackThreshold: 30
    });
  }

  // Custom cell renderer for Name column with link
  function nameCellRenderer(params) {
    if (!params.data) return '';
    const link = document.createElement('a');
    link.href = `/record/${params.data.id}`;
    link.textContent = params.value;
    return link;
  }

  // Custom cell renderer for Categories column
  function categoriesCellRenderer(params) {
    if (!params.data) return '';
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
    if (!params.data) return '';
    const organizations = params.value || [];
    if (organizations.length === 0) {
      const span = document.createElement('span');
      span.className = 'text-muted';
      span.textContent = '-';
      return span;
    }

    const container = document.createElement('span');
    organizations.forEach((org, index) => {
      if (index > 0) {
        container.appendChild(document.createTextNode(', '));
      }
      const link = document.createElement('a');
      link.href = `/browse?ror=${org.id}`;
      link.textContent = org.name;
      container.appendChild(link);
    });
    return container;
  }

  // Custom cell renderer for Created column
  function createdCellRenderer(params) {
    if (!params.data) return '';
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
    if (!params.data) return '';
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
      filter: 'agTextColumnFilter'
    },
    {
      field: 'uploaderName',
      headerName: 'Author',
      filter: 'agTextColumnFilter'
    },
    {
      field: 'categories',
      headerName: 'Categories',
      cellRenderer: categoriesCellRenderer,
      valueFormatter: params => {
        const categories = params.value || [];
        return categories.map(cat => cat.name).join(', ') || '-';
      },
      filter: false
    },
    {
      field: 'organizations',
      headerName: 'Organizations',
      cellRenderer: organizationsCellRenderer,
      valueFormatter: params => {
        const organizations = params.value || [];
        return organizations.map(org => org.name).join(', ') || '-';
      },
      filter: false
    },
    {
      field: 'downloadCount',
      headerName: 'Downloads',
      filter: 'agNumberColumnFilter',
      maxWidth: 120
    },
    {
      field: 'createdAt',
      headerName: 'Created',
      cellRenderer: createdCellRenderer,
      valueFormatter: params => {
        return formatRelativeTime(params.value);
      },
      filter: false,
      maxWidth: 130
    },
    {
      headerName: 'Actions',
      cellRenderer: actionsCellRenderer,
      filter: false,
      sortable: false,
      minWidth: 180
    }
  ];

  // Grid options following AG Grid official pattern
  // styleNonce is used to avoid 'unsafe-inline' in CSP for AG Grid styles
  const gridOptions = {
    columnDefs: columnDefs,
    defaultColDef: {
      flex: 1,
      minWidth: 100,
      filter: true,
      sortable: true,
      suppressHeaderMenuButton: true,
      suppressHeaderContextMenu: true,
      resizable: true,
      floatingFilter: true
    },
    domLayout: 'autoHeight',
    pagination: true,
    paginationPageSize: 10,
    paginationPageSizeSelector: [10, 20, 30, 50],
    animateRows: true,
    styleNonce: styleNonce,
    // Server-side row model for API-based data fetching
    rowModelType: 'infinite',
    cacheBlockSize: 10,
    maxBlocksInCache: 10,
    datasource: {
      getRows: async function (params) {
        const page = Math.floor(params.startRow / 10) + 1;
        const pageSize = params.endRow - params.startRow;

        // Build query string from current URL params
        const urlParams = new URLSearchParams(window.location.search);
        urlParams.set('short', '1');
        urlParams.set('page', page.toString());
        urlParams.set('pageSize', pageSize.toString());

        // Add sort parameters if present
        if (params.sortModel && params.sortModel.length > 0) {
          const sortModel = params.sortModel[0];
          urlParams.set('sortBy', sortModel.colId);
          urlParams.set('sortOrder', sortModel.sort);
        }

        // Add filter parameters if present
        if (params.filterModel) {
          // Handle text filters (name, uploaderName)
          if (params.filterModel.name) {
            const nameFilter = params.filterModel.name;
            if (nameFilter.filter) {
              urlParams.set('filterName', nameFilter.filter);
              urlParams.set('filterNameType', nameFilter.type || 'contains');
            }
          }

          if (params.filterModel.uploaderName) {
            const authorFilter = params.filterModel.uploaderName;
            if (authorFilter.filter) {
              urlParams.set('filterAuthor', authorFilter.filter);
              urlParams.set('filterAuthorType', authorFilter.type || 'contains');
            }
          }

          // Handle number filter (downloadCount)
          if (params.filterModel.downloadCount) {
            const downloadFilter = params.filterModel.downloadCount;
            if (downloadFilter.filter !== undefined) {
              urlParams.set('filterDownloads', downloadFilter.filter);
              urlParams.set('filterDownloadsType', downloadFilter.type || 'equals');
            }
            // Handle range filters (from/to)
            if (downloadFilter.filterTo !== undefined) {
              urlParams.set('filterDownloadsTo', downloadFilter.filterTo);
            }
          }
        }

        try {
          const response = await fetch(`/browse?${urlParams.toString()}`, {
            headers: {
              'Accept': 'application/json'
            }
          });

          if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
          }

          const data = await response.json();
          const records = data.records || [];
          const totalCount = data.pagination?.totalCount || 0;

          params.successCallback(records, totalCount);
        } catch (error) {
          console.error('Error fetching browse data:', error);
          params.failCallback();
        }
      }
    }
  };

  // Create the grid
  agGrid.createGrid(gridDiv, gridOptions);
}


// Moderation functionality
function initializeModerationButtons() {
  // Use event delegation for moderation buttons
  document.addEventListener('click', function (e) {
    const button = e.target.closest('.moderation-btn');
    if (!button) return;

    const recordId = button.getAttribute('data-record-id');
    const action = button.getAttribute('data-action');

    if (recordId && action) {
      moderateRecord(recordId, action, button);
    }
  });
}

async function moderateRecord(recordId, action, buttonElement) {
  // Get button elements
  const spinner = buttonElement.querySelector('.spinner-border');
  const btnText = buttonElement.querySelector('.btn-text');
  const originalText = btnText.textContent;

  // Disable all moderation buttons for this record
  const allButtons = document.querySelectorAll(`[data-record-id="${recordId}"]`);
  allButtons.forEach(btn => btn.disabled = true);

  // Show loading state
  spinner?.classList.remove('d-none');
  if (btnText) {
    const actionText = action.charAt(0).toUpperCase() + action.slice(1);
    btnText.textContent = `${actionText}ing...`;
  }

  try {
    const response = await fetch(`/api/v1/moderation/${recordId}`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ action })
    });

    if (response.ok) {
      // Show success toast
      const successToast = document.getElementById('successToast');
      const successToastBody = successToast?.querySelector('.toast-body');
      if (successToast && successToastBody) {
        const actionText = action.charAt(0).toUpperCase() + action.slice(1);
        successToastBody.textContent = `Record ${action}ed successfully!`;
        const toast = new bootstrap.Toast(successToast, {
          delay: 2000,
          autohide: true
        });

        // Reload page after toast hides
        successToast.addEventListener('hidden.bs.toast', function () {
          window.location.reload();
        }, { once: true });

        toast.show();
      } else {
        // Fallback reload if no toast
        setTimeout(() => {
          window.location.reload();
        }, 1000);
      }
    } else {
      // Show error toast
      const errorText = await response.text();
      const errorToast = document.getElementById('errorToast');
      const errorToastBody = document.getElementById('errorToastBody');
      if (errorToast && errorToastBody) {
        errorToastBody.textContent = errorText || `Failed to ${action} record. Please try again.`;
        const toast = new bootstrap.Toast(errorToast);
        toast.show();
      }
      console.error(`${action} failed:`, errorText);

      // Reset button state
      spinner?.classList.add('d-none');
      if (btnText) btnText.textContent = originalText;
      allButtons.forEach(btn => btn.disabled = false);
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
    console.error(`${action} error:`, error);

    // Reset button state
    spinner?.classList.add('d-none');
    if (btnText) btnText.textContent = originalText;
    allButtons.forEach(btn => btn.disabled = false);
  }
}

// Initialize moderation buttons when DOM is loaded
document.addEventListener('DOMContentLoaded', function () {
  initializeModerationButtons();
});
