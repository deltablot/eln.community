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

function getMainDataset(roCrateData) {
  if (!roCrateData || typeof roCrateData !== 'object') return {};

  graph = roCrateData['@graph'];
  if (!Array.isArray(graph)) return result;

  let hasPart;
  let hasPartId;
  let dataset;

  const crateNode = graph.find(node => {
    if (!node || typeof node !== 'object') return false;
    if (node['@id'] === './') {
        hasPart = node['hasPart'];
        hasPart.map(id => {
          if (id?.['@id']) hasPartId = id['@id'];
        });
    }
  });
  const nodes = graph.map(node => {
      if (hasPartId === node['@id']) dataset = node;
  });

  return dataset;
}

function renderBadge(badge) {
  return badge ? `<span class="badge bg-secondary small">${badge}</span>` : '';
}

function renderCardHeader(card, badge, title = '') {
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

function renderCustomFields(dataset) {
  return dataset.customFields.map(node => {
    const cardHeader = renderCardHeader(node['propertyID'], node['valueReference']);
    const body = `
      <dl class="row mb-0 mt-2 small">
        <dt class="col-sm-3 text-muted fw-medium">value</dt>
        <dd class="col-sm-9 text-break">${node['value']}</dd>
        ${node['description'] ? ` <dt class="col-sm-3 text-muted fw-medium">description</dt>
        <dd class="col-sm-9 text-break">${node['description']}</dd>` : ''}
      </dl>`;
      return renderCard(cardHeader, body);
  }).join('');
}

function renderSteps(dataset) {
  return dataset.steps.map(node => {
    const cardHeader = renderCardHeader(node['position'], node['creativeWorkStatus'], 'Step');
    const body = `
      <dl class="row mb-0 mt-2 small">
        <p>${node['text']}</p>
      </dl>`
    return renderCard(cardHeader, body);
  }).join('');
}

function renderMainText(dataset) {
  if (dataset.encodingFormat === 'text/markdown')
    return DOMPurify.sanitize(marked.parse(dataset.mainText));
  if (dataset.encodingFormat === 'text/html')
    return DOMPurify.sanitize(dataset.mainText);

  return dataset.mainText;
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
        URL: <a href="${dataset.URL}" target="_blank">${dataset.type} available on Elabftw</a>
      </div>
    `;
}

function renderAccordionSection(title, content) {
    return `
      <div class="accordion-body">
        <div class="fw-semibold mb-2">${title}</div>
         ${content ? content : 'No data available'}
       </div>
    `;
}

/**
 * Render the Main Text Block HTML
 * Displays title and content
 * in a collapsible accordion format
 *
 * @returns {string} - HTML string for the Main Text Block
 */
function renderData(dataset) {
  // Generate unique ID for accordion
  const accordionId = 'mainTextAccordion';
  const collapseId = 'mainTextCollapse';
  let mainText = renderMainText(dataset);
  if (!mainText)
    mainText = 'No data available';

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
          ${renderAccordionSection('Main Text', `<div class="text-break">${mainText}</div>`)}
          ${renderAccordionSection('Custom Fields', `${renderCustomFields(dataset)}`)}
          ${renderAccordionSection('Steps', `${renderSteps(dataset)}`)}
        </div>
      </div>
    </div>
  `;
}

function extractObject(graph, object) {
  let result;

  const graphId = graph.find((node) => {
    if (node['@id'] === object['@id']) {
      result = node;
    }
  });
  return result;
}

function extractArray(graph, array) {
  let result = [];

  const graphId = graph.find((node) => {
    const arrayId = array.map((id) => {
      if (node['@id'] === id['@id'] && node['propertyID'] !== 'elabftw_metadata') {
          result.push(node);
      }
    });
  });

  return result;
}

function extractSteps(graph, dataset) {
  let steps = extractArray(graph, dataset.step);

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

  const result = {
      author: null,
      title: null,
      encodingFormat: null,
      URL: null,
      type: null,
      status: null,
      tags: [],
      mainText: null,
      category: [],
      customFields: [],
      steps: [],
  };

  dataset.author ? result.author = extractObject(graph, dataset.author) : '';
  dataset.name ? result.title = dataset.name : '';
  dataset.encodingFormat ? result.encodingFormat = dataset.encodingFormat : '';
  dataset.url ? result.URL = dataset.url : '';
  dataset.genre ? result.type = dataset.genre : '';
  dataset.creativeWorkStatus ? result.status = dataset.creativeWorkStatus : '';
  dataset.keywords ? result.tags = dataset.keywords : '';
  dataset.text ? result.mainText = dataset.text : '';
  dataset.about ? result.category = extractObject(graph, dataset.about) : '';
  dataset.variableMeasured ? result.customFields = extractArray(graph, dataset.variableMeasured) : '';
  dataset.step ? result.steps = extractSteps(graph, dataset) : '';

  return result;
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
