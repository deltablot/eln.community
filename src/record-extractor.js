/**
 * Record Data Extractor Module
 *
 * Extracts and maps RO-Crate metadata to structured display blocks
 * for the redesigned record screen.
 */

import { marked } from "marked";
import DOMPurify from "dompurify";

/**
 * Extract graph['@id'] corresponding to the main dataset
 *
 * @param {Object} roCrateData - The complete RO-Crate JSON object
 * @returns {Object} - The node graph['@id'] with type 'Dataset'
 */

let graph = null;
const noData = 'No data available';

function getMainDataset(roCrateData) {
  if (!roCrateData || typeof roCrateData !== 'object') return {};

  graph = roCrateData['@graph'];
  if (!Array.isArray(graph)) return {};

  let hasPart;
  let hasPartId;
  let dataset;

  graph.find(node => {
    if (!node || typeof node !== 'object') return false;
    if (node['@id'] === './') {
        hasPart = node['hasPart'];
        hasPart.map(id => {
          if (id?.['@id']) hasPartId = id['@id'];
        });
    }
  });
  graph.map(node => {
      if (hasPartId === node['@id']) {
        dataset = node;
      }
  });

  return dataset;
}

function formatDateTime(value) {
    const date = new Date(value);
    const hasTime = value.includes('T');
    let options = {year: 'numeric', month: 'long', day: 'numeric' };

    if (hasTime) {
      options.hour = '2-digit';
      options.minute = '2-digit';
    }

    return date.toLocaleString('en-US', options);
}

function renderCardHeader(title, card, badge = '') {
    return `
      <div class="d-flex align-items-center flex-wrap gap-2">
        <strong>${title} ${card}</strong>
        ${badge ? `<span class="badge bg-secondary small">${badge}</span>` : ''}
      </div>`;
}

function renderCard(header, body) {
    return `
      <div class="card mb-2 border">
        <div class="card-body py-2 bg-light">
          ${header}
          ${body}
        </div>
      </div>`;
}

function renderSelect(value) {
  const values = Array.isArray(value) ? value : [value];

  return `
    <select name="choice" class="form-select">
      ${values.map(val => `<option value="${val}">${val}</option>`).join('')}
    </select>
  `;
}

function renderCheckbox(value) {
  const checked = value === 'on';

  return `
    <input class="form-check-input" type="checkbox" ${checked ? 'checked' : ''} disabled> (${checked ? 'checked' : 'unchecked'})
  `;
}

function renderLink(value, hrefValue = value) {
  const link = document.createElement('a');
  link.href = hrefValue;
  link.textContent = value;
  return link.outerHTML;
}

function renderField(node) {
  let ref = node['valueReference'];
  const value = node['value'];

  if (!value && ref !== 'checkbox') return noData;

  if (ref.startsWith('date')) return formatDateTime(value);

  switch (ref) {
    case 'url':
      return renderLink(value);
    case 'checkbox':
      return renderCheckbox(value);
    case 'email':
      return renderLink(value, `mailto:${value}`);
    case 'select':
      return renderSelect(value);
    case 'radio': {
      const radio = document.createElement('input');
      radio.type = 'radio';
      radio.name = value;
      radio.value = value;
      radio.setAttribute('checked', 'checked');
      return `${radio.outerHTML} ${value}`;
    }
    default:
      return value;
  }
}

function renderCustomFields(dataset) {
  return dataset.customFields.map(node => {
    const cardHeader = renderCardHeader(node['propertyID'], '', node['valueReference']);
    const body = `
      <dl class="row mb-0 mt-2 small">
        <dt class="col-sm-3 text-muted fw-medium">value</dt>
        <dd class="col-sm-9 text-break">${renderField(node)}</dd>
        ${node['unitText'] ? ` <dt class="col-sm-3 text-muted fw-medium">unit</dt>
        <dd class="col-sm-9 text-break">${node['unitText']}</dd>` : ''}
        ${node['description'] ? ` <dt class="col-sm-3 text-muted fw-medium">description</dt>
        <dd class="col-sm-9 text-break">${node['description']}</dd>` : ''}
      </dl>`;

      return renderCard(cardHeader, body);
  }).join('');
}

function renderSteps(dataset) {
  return dataset.steps.map(node => {
    const cardHeader = renderCardHeader('Step', node['position']);
    const body = `
      <dl class="row mb-0 mt-2 small">
        <p>${node['text']}</p>
      </dl>`

    return renderCard(cardHeader, body);
  }).join('');
}

function formatFileSize(size) {
  const bytes = Number(size);
  if (!bytes || isNaN(bytes)) return '';

  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let value = bytes;
  let i = 0;

  for (; value >= 1024 && i < units.length - 1; i++) {
      value /= 1024;
  }

  return `${value.toFixed(i ? 1 : 0)} ${units[i]}`;
}

function renderFiles(dataset) {
  return dataset.files.map(node => {
    const cardHeader = renderCardHeader('', node['name']);
    const body = `<dl class="row mb-0 mt-2 small"><p>${formatFileSize(node['contentSize'])}</p></dl>`

    return renderCard(cardHeader, body);
  }).join('');
}

function renderTab(html) {
  const template = document.createElement('template');
  template.innerHTML = html;

  template.content.querySelectorAll('table').forEach(table => {
    table.classList.add('table', 'w-auto');
  });

  template.content.querySelectorAll('td, th').forEach(cell => {
    cell.classList.add('border-bottom');
  });

  return template.innerHTML;
}

function renderMainText(dataset) {
  if (!dataset.mainText) return noData;

  let html = dataset.mainText;
  const format = dataset.encodingFormat;

  if (format === 'text/markdown')
    html = marked.parse(dataset.mainText);
  if (format === 'text/html' || format === 'text/markdown')
    return renderTab(DOMPurify.sanitize(html));

  return `<div class="text-break">${html}</div>`;
}

function renderCommonInfo(dataset) {
    return `
      <div class="accordion-body">
        Type: ${dataset.type}<br>
        Title: ${dataset.title}<br>
        Author: ${dataset.author['givenName']} ${dataset.author['familyName']}<br>
        Status: ${dataset.status ? dataset.status : ''}<br>
        Categories: ${dataset.category['name'] ? dataset.category['name'] : ''}<br>
        Tags: ${dataset.tags ? dataset.tags : ''}<br>
      </div>
    `;
}

function renderAccordionSection(title, content) {
    return `
      <div class="accordion-body">
        <div class="fw-semibold mb-2">${title}</div>
         ${content ? content : noData}
       </div>
    `;
}

function renderData(dataset) {
  const accordionId = 'mainTextAccordion';
  const collapseId = 'mainTextCollapse';
  const mainText = renderMainText(dataset);

  return `
    <div class="accordion mb-3" id="${accordionId}">
      <div class="accordion-item">
        <h2 class="accordion-header">
          <button class="accordion-button fw-semibold bg-light" type="button" data-bs-toggle="collapse" data-bs-target="#${collapseId}" aria-expanded="true" aria-controls="${collapseId}">
            <i class="bi bi-file-text me-2 text-secondary"></i>Summary
          </button>
        </h2>
        <div id="${collapseId}" class="accordion-collapse collapse show" data-bs-parent="#${accordionId}">
          ${renderCommonInfo(dataset)}
          ${renderAccordionSection('Main Text', `${mainText}`)}
          ${renderAccordionSection('Custom Fields', `${renderCustomFields(dataset)}`)}
          ${renderAccordionSection('Steps', `${renderSteps(dataset)}`)}
          ${renderAccordionSection('Files', `${renderFiles(dataset)}`)}
        </div>
      </div>
    </div>
  `;
}

function extractObjectFromDataset(graph, inputObject) {
  if (!inputObject) return '';

  return graph.find(node => node['@id'] === inputObject['@id']);
}

// ignoredPropertyID skips special nodes (like 'elabftw_metadata'),
// whose full metadata JSON value is not needed for display here.
function extractArrayFromDataset(graph, inputArray, ignoredPropertyID = '') {
  if (!inputArray) return [];

  let result = [];

  graph.find((node) => {
    inputArray.map((id) => {
      if (node['@id'] === id['@id'] && node['propertyID'] !== ignoredPropertyID)
          result.push(node);
    });
  });

  return result;
}

function extractSteps(graph, dataset) {
  let steps = extractArrayFromDataset(graph, dataset.step);

  return steps.map(step => {
    const directionId = step.itemListElement?.['@id'];
    const direction = graph.find(element => element?.['@id'] === directionId);

    return {
      position: step.position || '',
      creativeWorkStatus: step?.creativeWorkStatus || '',
      text: direction?.text || '',
    };
  });
}

function extractRecordData(roCrateData) {
  if (!roCrateData || typeof roCrateData !== 'object') return {};

  let dataset = getMainDataset(roCrateData);
  if (!dataset) return {};

  return {
      author: extractObjectFromDataset(graph, dataset.author),
      title: dataset.name,
      encodingFormat: dataset.encodingFormat,
      type: dataset.genre,
      status: dataset.creativeWorkStatus,
      files: extractArrayFromDataset(graph, dataset.hasPart),
      tags: dataset.keywords,
      mainText: dataset.text,
      category: extractObjectFromDataset(graph, dataset.about),
      customFields: extractArrayFromDataset(graph, dataset.variableMeasured, 'elabftw_metadata'),
      steps: extractSteps(graph, dataset),
  };
}

// Export functions for use in other modules
if (typeof module !== 'undefined' && module.exports) {
  module.exports = {
    extractRecordData,
    renderData,
  };
}

// Also make available globally for browser use
if (typeof window !== 'undefined') {
  window.RecordExtractor = {
    extractRecordData,
    renderData,
  };
}
