/**
 * Record Data Extractor Module
 *
 * Extracts and maps RO-Crate metadata to structured display blocks
 * for the redesigned record screen.
 */


/**
 * Pre-process function to extract graph['@id']
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

function renderCustomFields(dataset) {
  return dataset.customFields.map(node =>
    `<div class="card mb-2 border">
       <div class="card-body py-2 bg-light">
         <div class="d-flex align-items-center flex-wrap gap-2">
           <strong>${node['propertyID']}</strong>
             <span class="badge bg-secondary small">${node['valueReference']}</span>
         </div>
         <dl class="row mb-0 mt-2 small">
           <dt class="col-sm-3 text-muted fw-medium">value</dt>
           <dd class="col-sm-9 text-break">${node['value']}</dd>
           ${node['description'] ? `
             <dt class="col-sm-3 text-muted fw-medium">description</dt>
             <dd class="col-sm-9 text-break">${node['description']}</dd>
           ` : ''}
          </dl>
        </div>
      </div>`
    ).join('');
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

  //  console.log("dans renderMainText\n", dataset);
  //  console.log("dans renderMainText, le text\n", dataset.mainText);
  //  console.log("dans renderMainText, les customFields\n", dataset.customFields);

  return `
    <div class="accordion mb-3" id="${accordionId}">
      <div class="accordion-item">
        <h2 class="accordion-header">
          <button class="accordion-button fw-semibold bg-light" type="button" data-bs-toggle="collapse" data-bs-target="#${collapseId}" aria-expanded="true" aria-controls="${collapseId}">
            <i class="bi bi-file-text me-2 text-secondary"></i>${dataset.title}
          </button>
        </h2>
        <div id="${collapseId}" class="accordion-collapse collapse show" data-bs-parent="#${accordionId}">
          <div class="accordion-body">
            Author: ${dataset.author['givenName']} ${dataset.author['familyName']}<br>
            Genre: ${dataset.genre}<br>
            Status: ${dataset.status}<br>
            Categories: ${dataset.category['name']}<br>
            Tags: ${dataset.tags}<br>
          </div>
        </div>
        <div id="${collapseId}" class="accordion-collapse collapse show" data-bs-parent="#${accordionId}">
          <div class="accordion-body">
            ${dataset.mainText}
          </div>
        </div>
        <div id="${collapseId}" class="accordion-collapse collapse show" data-bs-parent="#${accordionId}">
          <div class="accordion-body">
            ${renderCustomFields(dataset)}
          </div>
        </div>
      </div>
    </div>
  `;
}

function extractAuthor(graph, dataset) {
  let author;

  const graphId = graph.find((nodeGraph) => {
    if (nodeGraph['@id'] === dataset.author['@id']) {
      author = nodeGraph;
    };
  });
  return author;
}

// TO DO: edit if category can be an array
function extractCategories(graph, dataset) {
  let category;

  const graphId = graph.find((nodeGraph) => {
    if (nodeGraph['@id'] === dataset.about['@id']) {
      category = nodeGraph;
    };
  });
  return category;
}

function extractCustomFields(graph, dataset) {
  let customFields = [];

  const graphId = graph.find((nodeGraph) => {
    const customFieldsId = dataset.variableMeasured.map((nodeCustomField) => {
      if (nodeGraph['@id'] === nodeCustomField['@id'] && nodeGraph['propertyID'] !== 'elabftw_metadata') {
          customFields.push(nodeGraph);
      }
    });
  });
  return customFields;
}

function extractRecordData(roCrateData) {
  if (!roCrateData || typeof roCrateData !== 'object') return {};

  let dataset = getMainDataset(roCrateData);
//  console.log('dans extractRecordData roCrateData', roCrateData);
//  console.log('dans extractRecordData', dataset);
  if (!dataset) return {};

 // console.log('dans extractRecordData', roCrateData);
  // to do: aller cherher valeur du rocrate pour encodingFormat
  // utiliser ça au lieu de son objet : simplifier encore plus
  // use fallback si l'attribut n'est pas la
  const result = {
      author: null,
      title: null,
     // encodingFormat: "text/html",
      genre: null,
      status: null,
      tags: [],
      mainText: null,
      category: [],
      customFields: [],
      steps: [],
  };

  dataset.author ? result.author = extractAuthor(graph, dataset) : '';
  dataset.name ? result.title = dataset.name : '';
  dataset.genre ? result.genre = dataset.genre : '';
  dataset.creativeWorkStatus ? result.status = dataset.creativeWorkStatus : '';
  dataset.keywords ? result.tags = dataset.keywords : '';
  dataset.text ? result.mainText = dataset.text : '';
  dataset.about ? result.category = extractCategories(graph, dataset) : '';
  dataset.variableMeasured ? result.customFields = extractCustomFields(graph, dataset) : '';
//  console.log('dans extractRecordData', result.customFields);
//  console.log('dans extractRecordData', dataset);
//  console.log("dans extractRecordData, le text\n", result.mainText);

  return result;
}


















/**
 * UUID regex pattern for filtering technical identifiers
 */
const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/**
 * Check if a string is a UUID
 * @param {string} str - String to check
 * @returns {boolean} - True if string matches UUID pattern
 */
function isUUID(str) {
  return typeof str === 'string' && UUID_PATTERN.test(str);
}

/**
 * Extract owner information from RO-Crate metadata
 * Looks for author/creator fields and extracts name and ORCID
 *
 * @param {Object} rootDataset - The root dataset entity from RO-Crate
 * @param {Array} graph - The @graph array from RO-Crate
 * @returns {Object|null} - Owner object with name and optional orcid, or null
 */
function extractOwner1(rootDataset, graph) {
  if (!rootDataset) return null;

  // Try author first, then creator
  const authorRef = rootDataset.author || rootDataset.creator;
  if (!authorRef) return null;

  // Handle array of authors - take the first one
  const ref = Array.isArray(authorRef) ? authorRef[0] : authorRef;

  if (!ref) return null;

  // If it's a direct object with name
  if (typeof ref === 'object' && ref.name) {
    return {
      name: ref.name,
      orcid: ref['@id'] && ref['@id'].includes('orcid.org')
        ? ref['@id'].replace('https://orcid.org/', '')
        : undefined
    };
  }

  // If it's a reference, look up in graph
  if (ref['@id']) {
    const authorEntity = graph.find(e => e['@id'] === ref['@id']);
    if (authorEntity && authorEntity.name) {
      return {
        name: authorEntity.name,
        orcid: authorEntity['@id'] && authorEntity['@id'].includes('orcid.org')
          ? authorEntity['@id'].replace('https://orcid.org/', '')
          : undefined
      };
    }
  }

  // If author is a string (name directly)
  if (typeof ref === 'string' && !ref.startsWith('#') && !ref.startsWith('./')) {
    return { name: ref };
  }

  return null;
}

/**
 * Extract team/organization information from RO-Crate metadata
 * Looks for affiliation, sourceOrganization, or organization fields
 *
 * @param {Object} rootDataset - The root dataset entity from RO-Crate
 * @param {Array} graph - The @graph array from RO-Crate
 * @returns {Object|null} - Team object with name and optional rorId, or null
 */
function extractTeam(rootDataset, graph) {
  if (!rootDataset) return null;

  // Try to get author first to find affiliation
  const authorRef = rootDataset.author || rootDataset.creator;
  if (authorRef) {
    const ref = Array.isArray(authorRef) ? authorRef[0] : authorRef;

    // Look up author entity to find affiliation
    if (ref && ref['@id']) {
      const authorEntity = graph.find(e => e['@id'] === ref['@id']);
      if (authorEntity && authorEntity.affiliation) {
        const affiliationRef = Array.isArray(authorEntity.affiliation)
          ? authorEntity.affiliation[0]
          : authorEntity.affiliation;

        if (affiliationRef) {
          // Direct object with name
          if (typeof affiliationRef === 'object' && affiliationRef.name) {
            return {
              name: affiliationRef.name,
              rorId: extractRorId(affiliationRef['@id'])
            };
          }

          // Reference to another entity
          if (affiliationRef['@id']) {
            const orgEntity = graph.find(e => e['@id'] === affiliationRef['@id']);
            if (orgEntity && orgEntity.name) {
              return {
                name: orgEntity.name,
                rorId: extractRorId(orgEntity['@id'])
              };
            }
          }
        }
      }
    }
  }

  // Try sourceOrganization on root dataset
  if (rootDataset.sourceOrganization) {
    const orgRef = Array.isArray(rootDataset.sourceOrganization)
      ? rootDataset.sourceOrganization[0]
      : rootDataset.sourceOrganization;

    if (orgRef) {
      if (typeof orgRef === 'object' && orgRef.name) {
        return {
          name: orgRef.name,
          rorId: extractRorId(orgRef['@id'])
        };
      }

      if (orgRef['@id']) {
        const orgEntity = graph.find(e => e['@id'] === orgRef['@id']);
        if (orgEntity && orgEntity.name) {
          return {
            name: orgEntity.name,
            rorId: extractRorId(orgEntity['@id'])
          };
        }
      }
    }
  }

  return null;
}

/**
 * Extract ROR ID from a URL or identifier
 * @param {string} id - The identifier to extract ROR from
 * @returns {string|undefined} - The ROR ID or undefined
 */
function extractRorId(id) {
  if (!id || typeof id !== 'string') return undefined;

  // Match ROR URL pattern
  const rorMatch = id.match(/ror\.org\/([a-z0-9]+)/i);
  if (rorMatch) return rorMatch[1];

  return undefined;
}

/**
 * Extract tags from RO-Crate metadata
 * Looks for keywords or about fields
 *
 * @param {Object} rootDataset - The root dataset entity from RO-Crate
 * @param {Array} graph - The @graph array from RO-Crate
 * @returns {string[]} - Array of tag strings
 */
function extractTags(rootDataset, graph) {
  if (!rootDataset) return [];

  const tags = [];

  // Extract from keywords (can be string or array)
  if (rootDataset.keywords) {
    if (Array.isArray(rootDataset.keywords)) {
      rootDataset.keywords.forEach(kw => {
        if (typeof kw === 'string' && !isUUID(kw)) {
          tags.push(kw);
        }
      });
    } else if (typeof rootDataset.keywords === 'string') {
      // Keywords might be comma-separated
      rootDataset.keywords.split(',').forEach(kw => {
        const trimmed = kw.trim();
        if (trimmed && !isUUID(trimmed)) {
          tags.push(trimmed);
        }
      });
    }
  }

  // Extract from about field
  if (rootDataset.about) {
    const aboutRefs = Array.isArray(rootDataset.about) ? rootDataset.about : [rootDataset.about];
    aboutRefs.forEach(ref => {
      if (typeof ref === 'object' && ref.name && !isUUID(ref.name)) {
        tags.push(ref.name);
      } else if (ref && ref['@id']) {
        const entity = graph.find(e => e['@id'] === ref['@id']);
        if (entity && entity.name && !isUUID(entity.name)) {
          tags.push(entity.name);
        }
      }
    });
  }

  return [...new Set(tags)]; // Remove duplicates
}

/**
 * Extract start date from RO-Crate metadata
 * Looks for dateCreated or startDate fields
 *
 * @param {Object} rootDataset - The root dataset entity from RO-Crate
 * @returns {string|null} - ISO date string or null
 */
function extractStartDate(rootDataset) {
  if (!rootDataset) return null;

  // Try dateCreated first, then startDate
  const dateValue = rootDataset.dateCreated || rootDataset.startDate || rootDataset.datePublished;

  if (!dateValue) return null;

  // Return as-is if it's a string (should be ISO format)
  if (typeof dateValue === 'string') {
    return dateValue;
  }

  return null;
}

/**
 * Extract main text sections from RO-Crate metadata
 * Identifies entities by name containing "Introduction"
 *
 * @param {Array} graph - The @graph array from RO-Crate
 * @returns {Object} - Object with introduction
 */
function extractMainText1(graph) {
  const mainText = {
    introduction: null,
  };

  if (!Array.isArray(graph)) return mainText;

  graph.forEach(entity => {
    if (!entity || !entity.name || !entity.text) return;
 //   const name = entity.name.toLowerCase();
    mainText.introduction = entity.text;
  });

  return mainText;
}

/**
 * Extract file attachments from RO-Crate metadata
 * Looks for hasPart relationships with File type
 *
 * @param {Object} rootDataset - The root dataset entity from RO-Crate
 * @param {Array} graph - The @graph array from RO-Crate
 * @returns {Array} - Array of FileInfo objects
 */
function extractFiles(rootDataset, graph) {
  const files = [];

  if (!rootDataset || !rootDataset.hasPart) return files;

  const parts = Array.isArray(rootDataset.hasPart) ? rootDataset.hasPart : [rootDataset.hasPart];

  parts.forEach(partRef => {
    if (!partRef) return;

    let entity = null;

    // Direct reference
    if (partRef['@id']) {
      entity = graph.find(e => e['@id'] === partRef['@id']);
    } else if (typeof partRef === 'object') {
      entity = partRef;
    }

    if (!entity) return;

    // Check if it's a File type
    const types = Array.isArray(entity['@type']) ? entity['@type'] : [entity['@type']];
    if (types.includes('File') || types.includes('MediaObject')) {
      files.push({
        id: entity['@id'] || '',
        name: entity.name || entity['@id'] || 'Unknown file',
        size: entity.contentSize,
        mimeType: entity.encodingFormat || entity.fileFormat
      });
    }
  });

  return files;
}

/**
 * Extract custom fields from RO-Crate metadata
 *
 * @param {Array} graph - The @graph array from RO-Crate
 * @returns {Array} - Array of FileInfo objects
 */
function extractCustomFields1(graph) {
  if (!Array.isArray(graph)) return [];

  const customFields = graph.find((entity) =>
    entity?.['@type'] === 'PropertyValue'
  );
  if (!customFields) return [];

  let metadata;
  metadata = JSON.parse(customFields.value);

  const field = metadata.extra_fields;
  return Object.entries(field).map(([fieldName, fieldData]) => ({
      name: fieldName,
      value: fieldData?.value || '',
      type: fieldData?.type || '',
      description: fieldData?.description || '',
  }))
}

function renderCustomFields1(customFields) {
  if (!Array.isArray(customFields) || customFields.length === 0) return '';

  // Generate unique ID for accordion
  const accordionId = 'mainTextAccordion';
  const collapseId = 'mainTextCollapse';

  const customFieldsHtml = customFields.map(field =>
    `<div class="card mb-2 border">
       <div class="card-body py-2 bg-light">
         <div class="d-flex align-items-center flex-wrap gap-2">
           <strong>${escapeHtmlForRenderer(field.name)}</strong>
             ${field.type ? `<span class="badge bg-secondary small">${escapeHtmlForRenderer(field.type)}</span>` : ''}
         </div>

         <dl class="row mb-0 mt-2 small">
           <dt class="col-sm-3 text-muted fw-medium">value</dt>
           <dd class="col-sm-9 text-break">${escapeHtmlForRenderer(String(field.value))}</dd>

           ${field.description ? `
             <dt class="col-sm-3 text-muted fw-medium">description</dt>
             <dd class="col-sm-9 text-break">${escapeHtmlForRenderer(field.description)}</dd>
           ` : ''}
          </dl>
        </div>
      </div>`
    ).join('');

  return `
    <div class="accordion mb-3" id="${accordionId}">
      <div class="accordion-item">
        <h2 class="accordion-header">
          <button class="accordion-button fw-semibold bg-light" type="button" data-bs-toggle="collapse" data-bs-target="#${collapseId}" aria-expanded="true" aria-controls="${collapseId}">
            <i class="bi bi-file-text me-2 text-secondary"></i>CUSTOM FIELDS
          </button>
        </h2>
        <div id="${collapseId}" class="accordion-collapse collapse show" data-bs-parent="#${accordionId}">
          <div class="accordion-body">
            ${customFieldsHtml}
          </div>
        </div>
      </div>
    </div>
  `;
}

/**
 * Extract steps from RO-Crate metadata
 *
 * @param {Array} graph - The @graph array from RO-Crate
 * @returns {Array} - Array of FileInfo objects
 */
function extractSteps(graph) {
  if (!Array.isArray(graph)) return [];

  const steps = graph.filter(entity => entity?.['@type'] === 'HowToStep');
  if (!steps) return [];

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

function renderSteps(steps) {
  if (!Array.isArray(steps) || steps.length === 0) return '';

  const accordionId = 'mainTextAccordion';
  const collapseId = 'mainTextCollapse';

  const stepsHtml = steps.map(step =>
      `<div class="card mb-2 border">
            <div class="card-body py-2 bg-light">
              <div class="d-flex align-items-center flex-wrap gap-2">
                 <strong>Step ${escapeHtmlForRenderer(String(step.position))}</strong>
                 ${step.creativeWorkStatus ? `<span class="badge bg-secondary small">${escapeHtmlForRenderer(step.creativeWorkStatus)}</span>` : ''}
              </div>
              <dl class="row mb-0 mt-2 small">
                <p>${escapeHtmlForRenderer(step?.text)}</p>
              </dl>
            </div>
          </div>`).join('');

  return `
    <div class="accordion mb-3" id="${accordionId}">
      <div class="accordion-item">
        <h2 class="accordion-header">
          <button class="accordion-button fw-semibold bg-light" type="button" data-bs-toggle="collapse" data-bs-target="#${collapseId}" aria-expanded="true" aria-controls="${collapseId}">
            <i class="bi bi-file-text me-2 text-secondary"></i>STEPS
          </button>
        </h2>
        <div id="${collapseId}" class="accordion-collapse collapse show" data-bs-parent="#${accordionId}">
          <div class="accordion-body">
            ${stepsHtml}
          </div>
        </div>
      </div>
    </div>
  `;
}

/**
 * Extract links from RO-Crate metadata
 * Looks for mentions, isBasedOn, and citation fields
 *
 * @param {Object} rootDataset - The root dataset entity from RO-Crate
 * @param {Array} graph - The @graph array from RO-Crate
 * @returns {Object} - Object with experimentLinks and resourceLinks arrays
 */
function extractLinks(rootDataset, graph) {
  const links = {
    experimentLinks: [],
    resourceLinks: []
  };

  if (!rootDataset) return links;

  // Helper to process link references
  const processLinkRef = (ref, type) => {
    if (!ref) return;

    let entity = null;
    let url = null;

    if (typeof ref === 'string') {
      url = ref;
    } else if (ref['@id']) {
      entity = graph.find(e => e['@id'] === ref['@id']);
      url = ref['@id'];
    } else if (typeof ref === 'object') {
      entity = ref;
      url = ref.url || ref['@id'];
    }

    const linkInfo = {
      id: entity?.['@id'] || url || '',
      name: entity?.name || url || 'Unknown link',
      url: entity?.url || url,
      type: type
    };

    if (type === 'experiment') {
      links.experimentLinks.push(linkInfo);
    } else {
      links.resourceLinks.push(linkInfo);
    }
  };

  // Process mentions as experiment links
  if (rootDataset.mentions) {
    const mentions = Array.isArray(rootDataset.mentions) ? rootDataset.mentions : [rootDataset.mentions];
    mentions.forEach(ref => processLinkRef(ref, 'experiment'));
  }

  // Process isBasedOn and citation as resource links
  if (rootDataset.isBasedOn) {
    const basedOn = Array.isArray(rootDataset.isBasedOn) ? rootDataset.isBasedOn : [rootDataset.isBasedOn];
    basedOn.forEach(ref => processLinkRef(ref, 'resource'));
  }

  if (rootDataset.citation) {
    const citations = Array.isArray(rootDataset.citation) ? rootDataset.citation : [rootDataset.citation];
    citations.forEach(ref => processLinkRef(ref, 'resource'));
  }

  return links;
}

/**
 * Collect unmapped entities from RO-Crate metadata
 * Returns entities that don't match known field mappings
 *
 * @param {Array} graph - The @graph array from RO-Crate
 * @param {Object} rootDataset - The root dataset entity
 * @param {Object} extractedData - Already extracted data to identify mapped entities
 * @returns {Array} - Array of unmapped RO-Crate entities
 */
function collectUnmappedEntities(graph, rootDataset, extractedData) {
  if (!Array.isArray(graph)) return [];

  // Build set of mapped entity IDs
  const mappedIds = new Set();

  // Root dataset is mapped
  if (rootDataset && rootDataset['@id']) {
    mappedIds.add(rootDataset['@id']);
  }

  // Add ro-crate-metadata.json
  mappedIds.add('ro-crate-metadata.json');

  // Author/creator entities
  const authorRef = rootDataset?.author || rootDataset?.creator;
  if (authorRef) {
    const refs = Array.isArray(authorRef) ? authorRef : [authorRef];
    refs.forEach(ref => {
      if (ref && ref['@id']) mappedIds.add(ref['@id']);
    });
  }

  // Organization entities
  graph.forEach(entity => {
    if (!entity) return;
    const types = Array.isArray(entity['@type']) ? entity['@type'] : [entity['@type']];
    if (types.includes('Organization') || types.includes('Person')) {
      mappedIds.add(entity['@id']);
    }
  });

  // File entities
  extractedData.extraFields?.attachedFiles?.forEach(file => {
    if (file.id) mappedIds.add(file.id);
  });

  // Main text entities (by name pattern)
  graph.forEach(entity => {
    if (!entity || !entity.name) return;
    const name = entity.name.toLowerCase();
  });

  // Link entities
  const allLinks = [
    ...(extractedData.extraFields?.experimentLinks || []),
    ...(extractedData.extraFields?.resourceLinks || [])
  ];
  allLinks.forEach(link => {
    if (link.id) mappedIds.add(link.id);
  });

  // Filter out mapped entities
  return graph.filter(entity => {
    if (!entity || !entity['@id']) return false;
    return !mappedIds.has(entity['@id']);
  });
}



/**
 * Main function to extract all record data from RO-Crate metadata
 *
 * @param {Object} roCrateData - The complete RO-Crate JSON object
 * @returns {Object} - ExtractedRecordData object with all structured data
    /*
 */
function extractRecordData1(roCrateData) {
  // Initialize result structure
//    console.log(roCrateData);
  const result = {
    commonInfo: {
      startDate: null,
      owner: null,
      team: null,
      tags: []
    },
    mainText: {
      introduction: null,
    },
    extraFields: {
      customFields: [],
      attachedFiles: [],
      steps: [],
      experimentLinks: [],
      resourceLinks: [],
      compounds: [],
      storage: [],
      permissions: {
        visibility: 'public',
        canWrite: 'owner'
      }
    },
    unmappedEntities: []
  };

  // Validate input
  if (!roCrateData || typeof roCrateData !== 'object') {
    return result;
  }

  const graph = roCrateData['@graph'];
  if (!Array.isArray(graph)) {
    return result;
  }

  // Find root dataset
  const rootDataset = graph.find(node => {
    if (!node || typeof node !== 'object') return false;
    if (node['@id'] === './') return true;
    const types = Array.isArray(node['@type']) ? node['@type'] : [node['@type']];
    return types.includes('Dataset');
  });
    console.log(rootDataset);

  // Extract common info
  result.commonInfo.owner = extractOwner1(rootDataset, graph);
  result.commonInfo.team = extractTeam(rootDataset, graph);
  result.commonInfo.tags = extractTags(rootDataset, graph);
  result.commonInfo.startDate = extractStartDate(rootDataset);

  // Extract main text
  result.mainText = extractMainText(graph);

  // Extract extra fields
  result.extraFields.customFields = extractCustomFields1(graph);
  result.extraFields.attachedFiles = extractFiles(rootDataset, graph);
  result.extraFields.steps = extractSteps(graph);
  const links = extractLinks(rootDataset, graph);
  result.extraFields.experimentLinks = links.experimentLinks;
  result.extraFields.resourceLinks = links.resourceLinks;

  // Collect unmapped entities
  result.unmappedEntities = collectUnmappedEntities(graph, rootDataset, result);

  return result;
}

/**
 * Format a date string into human-readable format
 * @param {string|null} dateStr - ISO date string or null
 * @returns {string} - Formatted date string or empty string
 */
function formatDate(dateStr) {
  if (!dateStr) return '';

  try {
    const date = new Date(dateStr);
    if (isNaN(date.getTime())) {
      return ''; // Invalid date
    }

    // Format as "Month DD, YYYY" (e.g., "January 15, 2024")
    const options = { year: 'numeric', month: 'long', day: 'numeric' };
    return date.toLocaleDateString('en-US', options);
  } catch (e) {
    return '';
  }
}

/**
 * Escape HTML special characters to prevent XSS
 * @param {string} text - Text to escape
 * @returns {string} - Escaped text
 */
function escapeHtmlForRenderer(text) {
  if (typeof text !== 'string') return '';

  // For Node.js environment (testing), use string replacement
  if (typeof document === 'undefined') {
    return text
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#039;');
  }

  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

/**
 * Filter out UUID patterns from a string
 * @param {string} str - String to filter
 * @returns {string} - String with UUIDs removed
 */
function filterUUIDs(str) {
  if (typeof str !== 'string') return str;
  // Replace UUIDs with empty string, handling various formats
  return str.replace(/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/gi, '').trim();
}

/**
 * Check if a value contains only UUID(s) and should be hidden
 * @param {string} str - String to check
 * @returns {boolean} - True if the string is essentially just UUIDs
 */
function isOnlyUUID(str) {
  if (typeof str !== 'string') return false;
  const filtered = filterUUIDs(str);
  return filtered.length === 0 || filtered === str.replace(/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/gi, '').trim().length === 0;
}

/**
 * Render the Common Info Block HTML
 * Displays owner, team, tags, and date in a structured format
 *
 * @param {Object} commonInfo - The commonInfo object from ExtractedRecordData
 * @param {string|null} commonInfo.startDate - ISO date string
 * @param {Object|null} commonInfo.owner - Owner object with name and optional orcid
 * @param {Object|null} commonInfo.team - Team object with name and optional rorId
 * @param {string[]} commonInfo.tags - Array of tag strings
 * @returns {string} - HTML string for the Common Info Block
 */
function renderCommonInfoBlock(commonInfo) {
  if (!commonInfo) {
    return '<div><p class="text-muted">No information available</p></div>';
  }

  const { startDate, owner, team, tags } = commonInfo;

  let html = '<div>';

  // Date row
  if (startDate) {
    const formattedDate = formatDate(startDate);
    if (formattedDate) {
      html += `<div class="mb-2">
        <i class="bi bi-calendar3 me-2 text-secondary"></i>
        <span class="text-muted">Started on</span>
        <span class="fw-medium">${escapeHtmlForRenderer(formattedDate)}</span>
      </div>`;
    }
  }

  // Owner row
  if (owner && owner.name && !isOnlyUUID(owner.name)) {
    const ownerName = filterUUIDs(owner.name);
    html += `<div class="d-flex align-items-center flex-wrap mb-2">
      <i class="bi bi-person me-2 text-secondary"></i>
      <span class="text-muted me-2">Owner</span>
      <span>`;

    if (owner.orcid) {
      html += `<a href="https://orcid.org/${escapeHtmlForRenderer(owner.orcid)}" target="_blank" rel="noopener noreferrer" class="text-decoration-none">${escapeHtmlForRenderer(ownerName)}</a>`;
    } else {
      html += escapeHtmlForRenderer(ownerName);
    }

    html += `</span></div>`;
  }

  // Team row
  if (team && team.name && !isOnlyUUID(team.name)) {
    const teamName = filterUUIDs(team.name);
    html += `<div class="d-flex align-items-center flex-wrap mb-2">
      <i class="bi bi-people me-2 text-secondary"></i>
      <span class="text-muted me-2">Team</span>
      <span>`;

    if (team.rorId) {
      html += `<a href="https://ror.org/${escapeHtmlForRenderer(team.rorId)}" target="_blank" rel="noopener noreferrer" class="text-decoration-none">${escapeHtmlForRenderer(teamName)}</a>`;
    } else {
      html += escapeHtmlForRenderer(teamName);
    }

    html += `</span></div>`;
  }

  // Tags row - filter out UUIDs from tags
  const filteredTags = (tags || []).filter(tag => {
    if (typeof tag !== 'string') return false;
    return !isUUID(tag) && !isOnlyUUID(tag) && tag.trim().length > 0;
  }).map(tag => filterUUIDs(tag)).filter(tag => tag.length > 0);

  if (filteredTags.length > 0) {
    html += `<div class="d-flex align-items-center flex-wrap mb-2">
      <i class="bi bi-tags me-2 text-secondary"></i>
      <span class="text-muted me-2">Tags</span>
      <div class="d-inline-flex flex-wrap gap-1">`;

    filteredTags.forEach(tag => {
      html += `<span class="badge bg-primary">${escapeHtmlForRenderer(tag)}</span>`;
    });

    html += `</div></div>`;
  }

  html += '</div>';

  return html;
}

/**
 * Sanitize HTML content to remove dangerous elements and attributes
 * This prevents XSS attacks when rendering user-provided HTML content
 *
 * @param {string} html - Raw HTML string to sanitize
 * @returns {string} - Sanitized HTML string safe for rendering
 */
function sanitizeHTML(html) {
  if (typeof html !== 'string') return '';

  if (typeof document === 'undefined') {
    // Basic sanitization for server-side: escape HTML entities
    return html
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#039;');
  }

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
      // Remove javascript: URLs
      if ((name === 'href' || name === 'src') && attr.value.toLowerCase().trim().startsWith('javascript:')) {
        el.removeAttribute(attr.name);
      }
    });
  });

  return doc.body.innerHTML;
}

/**
 * Render a single main text section with heading
 *
 * @param {string} title - Section heading title
 * @param {string|null} content - HTML content for the section
 * @returns {string} - HTML string for the section, or empty string if no content
 */
function renderMainTextSection(title, content) {
  if (!content) return '';

  const sanitizedContent = sanitizeHTML(content);
  const escapedTitle = escapeHtmlForRenderer(title);

  return `
    <div class="mb-4 pb-3 border-bottom">
      <h5 class="fw-semibold mb-3 ps-3 border-start border-primary border-3">${escapedTitle}</h5>
      <div class="bg-white border rounded p-3 overflow-auto">
        ${sanitizedContent}
      </div>
    </div>
  `;
}

/**
 * Render the Main Text Block HTML
 * Displays Introduction
 * in a collapsible accordion format
 *
 * @param {Object} mainText - The mainText object from ExtractedRecordData
 * @param {string|null} mainText.introduction - Introduction content
 * @returns {string} - HTML string for the Main Text Block
 */
function renderMainTextBlock1(mainText) {
  if (!mainText) {
    mainText = {
      introduction: null,
    };
  }

  const { introduction } = mainText;

  // Check if there's any content to display
  const hasContent = introduction;

  // Build the sections content
  let sectionsHtml = '';
  sectionsHtml += renderMainTextSection('Introduction', introduction);

  // If no content, show a message
  if (!hasContent) {
    sectionsHtml = '<p class="text-muted">No main text content available</p>';
  }

  // Generate unique ID for accordion
  const accordionId = 'mainTextAccordion';
  const collapseId = 'mainTextCollapse';

  return `
    <div class="accordion mb-3" id="${accordionId}">
      <div class="accordion-item">
        <h2 class="accordion-header">
          <button class="accordion-button fw-semibold bg-light" type="button" data-bs-toggle="collapse" data-bs-target="#${collapseId}" aria-expanded="true" aria-controls="${collapseId}">
            <i class="bi bi-file-text me-2 text-secondary"></i>MAIN TEXT
          </button>
        </h2>
        <div id="${collapseId}" class="accordion-collapse collapse show" data-bs-parent="#${accordionId}">
          <div class="accordion-body">
            ${sectionsHtml}
          </div>
        </div>
      </div>
    </div>
  `;
}

/**
 * Format file size in human-readable format
 * @param {number|string|undefined} size - File size in bytes
 * @returns {string} - Formatted size string (e.g., "1.5 MB")
 */
function formatFileSize(size) {
  if (size === undefined || size === null) return '';

  const bytes = typeof size === 'string' ? parseInt(size, 10) : size;
  if (isNaN(bytes)) return '';

  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let unitIndex = 0;
  let value = bytes;

  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex++;
  }

  return `${value.toFixed(unitIndex > 0 ? 1 : 0)} ${units[unitIndex]}`;
}

/**
 * Render the attached files subsection
 * @param {Array} files - Array of FileInfo objects
 * @returns {string} - HTML string for the files subsection
 */
function renderAttachedFilesSubsection(files) {
  const fileCount = files ? files.length : 0;

  let html = `
    <div class="mb-3 pb-3 border-bottom">
      <h6 class="fw-semibold text-secondary mb-3">
        <i class="bi bi-paperclip me-2"></i>ATTACHED FILES (${fileCount})
      </h6>`;

  if (fileCount === 0) {
    html += '<p class="text-muted ms-4">No files attached</p>';
  } else {
    html += '<ul class="list-unstyled ms-4">';
    files.forEach(file => {
      const fileName = escapeHtmlForRenderer(file.name || 'Unknown file');
      const fileSize = file.size ? ` (${formatFileSize(file.size)})` : '';
      const mimeType = file.mimeType ? ` - ${escapeHtmlForRenderer(file.mimeType)}` : '';

      html += `<li class="py-1">
        <i class="bi bi-file-earmark me-2 text-secondary"></i>
        <span>${fileName}</span>
        <span class="text-muted small">${fileSize}${mimeType}</span>
      </li>`;
    });
    html += '</ul>';
  }

  html += '</div>';
  return html;
}

/**
 * Render a single link item
 * @param {Object} link - LinkInfo object
 * @returns {string} - HTML string for the link item
 */
function renderLinkItem(link) {
  const linkName = escapeHtmlForRenderer(link.name || link.url || 'Unknown link');
  const linkUrl = link.url || link.id;

  if (linkUrl && !isUUID(linkUrl)) {
    return `<li class="py-1">
      <i class="bi bi-link-45deg me-2 text-secondary"></i>
      <a href="${escapeHtmlForRenderer(linkUrl)}" target="_blank" rel="noopener noreferrer" class="text-decoration-none">${linkName}</a>
    </li>`;
  }

  return `<li class="py-1">
    <i class="bi bi-link-45deg me-2 text-secondary"></i>
    <span>${linkName}</span>
  </li>`;
}

/**
 * Render the links subsection
 * @param {Array} experimentLinks - Array of experiment LinkInfo objects
 * @param {Array} resourceLinks - Array of resource LinkInfo objects
 * @returns {string} - HTML string for the links subsection
 */
function renderLinksSubsection(experimentLinks, resourceLinks) {
  const expLinks = experimentLinks || [];
  const resLinks = resourceLinks || [];
  const totalLinks = expLinks.length + resLinks.length;

  let html = `
    <div class="mb-3 pb-3 border-bottom">
      <h6 class="fw-semibold text-secondary mb-3">
        <i class="bi bi-link me-2"></i>LINKS (${totalLinks})
      </h6>`;

  if (totalLinks === 0) {
    html += '<p class="text-muted ms-4">No links available</p>';
  } else {
    // Experiment Links
    if (expLinks.length > 0) {
      html += `<div class="ms-4 mb-2">
        <span class="fw-medium text-muted">Experiment Links:</span>
        <ul class="list-unstyled ms-2">`;
      expLinks.forEach(link => {
        html += renderLinkItem(link);
      });
      html += '</ul></div>';
    }

    // Resource Links
    if (resLinks.length > 0) {
      html += `<div class="ms-4 mb-2">
        <span class="fw-medium text-muted">Resource Links:</span>
        <ul class="list-unstyled ms-2">`;
      resLinks.forEach(link => {
        html += renderLinkItem(link);
      });
      html += '</ul></div>';
    }
  }

  html += '</div>';
  return html;
}

/**
 * Render the compounds subsection
 * @param {Array} compounds - Array of CompoundInfo objects
 * @returns {string} - HTML string for the compounds subsection, or empty string if no compounds
 */
function renderCompoundsSubsection(compounds) {
  if (!compounds || compounds.length === 0) return '';

  let html = `
    <div class="mb-3 pb-3 border-bottom">
      <h6 class="fw-semibold text-secondary mb-3">
        <i class="bi bi-droplet me-2"></i>COMPOUNDS (${compounds.length})
      </h6>
      <ul class="list-unstyled ms-4">`;

  compounds.forEach(compound => {
    const name = escapeHtmlForRenderer(compound.name || 'Unknown compound');
    const formula = compound.formula ? ` (${escapeHtmlForRenderer(compound.formula)})` : '';

    html += `<li class="py-1">
      <i class="bi bi-hexagon me-2 text-secondary"></i>
      <span>${name}</span>
      <span class="text-muted small">${formula}</span>
    </li>`;
  });

  html += '</ul></div>';
  return html;
}

/**
 * Render the storage subsection
 * @param {Array} storage - Array of StorageInfo objects
 * @returns {string} - HTML string for the storage subsection, or empty string if no storage info
 */
function renderStorageSubsection(storage) {
  if (!storage || storage.length === 0) return '';

  let html = `
    <div class="mb-3 pb-3 border-bottom">
      <h6 class="fw-semibold text-secondary mb-3">
        <i class="bi bi-box me-2"></i>STORAGE (${storage.length})
      </h6>
      <ul class="list-unstyled ms-4">`;

  storage.forEach(item => {
    const name = escapeHtmlForRenderer(item.name || 'Unknown storage');
    const location = item.location ? ` - ${escapeHtmlForRenderer(item.location)}` : '';

    html += `<li class="py-1">
      <i class="bi bi-archive me-2 text-secondary"></i>
      <span>${name}</span>
      <span class="text-muted small">${location}</span>
    </li>`;
  });

  html += '</ul></div>';
  return html;
}

/**
 * Render the permissions subsection
 * @param {Object} permissions - PermissionInfo object
 * @returns {string} - HTML string for the permissions subsection
 */
function renderPermissionsSubsection(permissions) {
  const perms = permissions || { visibility: 'public', canWrite: 'owner' };

  const visibility = escapeHtmlForRenderer(perms.visibility || 'public');
  const canWrite = escapeHtmlForRenderer(perms.canWrite || 'owner');

  return `
    <div class="mb-3">
      <h6 class="fw-semibold text-secondary mb-3">
        <i class="bi bi-shield-lock me-2"></i>PERMISSIONS
      </h6>
      <div class="ms-4">
        <div class="mb-1">
          <i class="bi bi-eye me-2 text-secondary"></i>
          <span class="text-muted me-2">Visibility:</span>
          <span>${visibility}</span>
        </div>
        <div class="mb-1">
          <i class="bi bi-pencil me-2 text-secondary"></i>
          <span class="text-muted me-2">Can Write:</span>
          <span>${canWrite}</span>
        </div>
      </div>
    </div>
  `;
}

/**
 * Render the Extra Fields Block HTML
 * Displays attached files, links, compounds, storage, and permissions
 * in a collapsible accordion format
 *
 * @param {Object} extraFields - The extraFields object from ExtractedRecordData
 * @param {Array} extraFields.attachedFiles - Array of FileInfo objects
 * @param {Array} extraFields.experimentLinks - Array of experiment LinkInfo objects
 * @param {Array} extraFields.resourceLinks - Array of resource LinkInfo objects
 * @param {Array} extraFields.compounds - Array of CompoundInfo objects
 * @param {Array} extraFields.storage - Array of StorageInfo objects
 * @param {Object} extraFields.permissions - PermissionInfo object
 * @returns {string} - HTML string for the Extra Fields Block
 */
function renderExtraFieldsBlock(extraFields) {
  if (!extraFields) {
    extraFields = {
      attachedFiles: [],
      experimentLinks: [],
      resourceLinks: [],
      compounds: [],
      storage: [],
      permissions: { visibility: 'public', canWrite: 'owner' }
    };
  }

  const {
    attachedFiles,
    experimentLinks,
    resourceLinks,
    compounds,
    storage,
    permissions
  } = extraFields;

  // Build the subsections content
  let subsectionsHtml = '';
  subsectionsHtml += renderAttachedFilesSubsection(attachedFiles);
  subsectionsHtml += renderLinksSubsection(experimentLinks, resourceLinks);
  subsectionsHtml += renderCompoundsSubsection(compounds);
  subsectionsHtml += renderStorageSubsection(storage);
  subsectionsHtml += renderPermissionsSubsection(permissions);

  // Generate unique ID for accordion
  const accordionId = 'extraFieldsAccordion';
  const collapseId = 'extraFieldsCollapse';

  return `
    <div class="accordion mb-3" id="${accordionId}">
      <div class="accordion-item">
        <h2 class="accordion-header">
          <button class="accordion-button collapsed fw-semibold bg-light" type="button" data-bs-toggle="collapse" data-bs-target="#${collapseId}" aria-expanded="false" aria-controls="${collapseId}">
            <i class="bi bi-list-ul me-2 text-secondary"></i>EXTRA FIELDS
          </button>
        </h2>
        <div id="${collapseId}" class="accordion-collapse collapse" data-bs-parent="#${accordionId}">
          <div class="accordion-body">
            ${subsectionsHtml}
          </div>
        </div>
      </div>
    </div>
  `;
}

/**
 * Render a single unmapped entity
 * @param {Object} entity - RO-Crate entity object
 * @returns {string} - HTML string for the entity
 */
function renderUnmappedEntity(entity) {
  if (!entity) return '';

  const entityId = entity['@id'] || 'Unknown';
  const entityType = Array.isArray(entity['@type']) ? entity['@type'].join(', ') : (entity['@type'] || 'Unknown');
  const entityName = entity.name || entityId;

  // Skip if the ID is just a UUID with no meaningful name
  if (isUUID(entityId) && (!entity.name || isUUID(entity.name))) {
    return '';
  }

  let html = `<div class="card mb-2 border">
    <div class="card-body py-2 bg-light">
      <div class="d-flex align-items-center flex-wrap gap-2">
        <strong>${escapeHtmlForRenderer(entityName)}</strong>
        <span class="badge bg-secondary small">${escapeHtmlForRenderer(entityType)}</span>
      </div>`;

  // Display other properties (excluding @id, @type, name)
  const excludeKeys = ['@id', '@type', 'name'];
  const otherProps = Object.keys(entity).filter(key => !excludeKeys.includes(key));

  if (otherProps.length > 0) {
    html += '<dl class="row mb-0 mt-2 small">';
    otherProps.forEach(key => {
      const value = entity[key];
      let displayValue;

      if (typeof value === 'object') {
        displayValue = JSON.stringify(value);
      } else {
        displayValue = String(value);
      }

      // Skip UUID-only values
      if (isUUID(displayValue)) return;

      html += `<dt class="col-sm-3 text-muted fw-medium">${escapeHtmlForRenderer(key)}</dt>
        <dd class="col-sm-9 text-break">${escapeHtmlForRenderer(displayValue)}</dd>`;
    });
    html += '</dl>';
  }

  html += '</div></div>';
  return html;
}

/**
 * Render the Other Metadata section for unmapped entities
 * Displays RO-Crate entities that don't match known field mappings
 *
 * @param {Array} unmappedEntities - Array of RO-Crate entity objects
 * @returns {string} - HTML string for the Other Metadata section
 */
function renderOtherMetadata(unmappedEntities) {
  if (!unmappedEntities || !Array.isArray(unmappedEntities)) {
    return '';
  }

  // Filter out entities that are just UUIDs with no meaningful content
  const meaningfulEntities = unmappedEntities.filter(entity => {
    if (!entity) return false;
    const entityId = entity['@id'] || '';
    const entityName = entity.name || '';

    // Keep if has a meaningful name that's not a UUID
    if (entityName && !isUUID(entityName)) return true;

    // Keep if has a meaningful ID that's not a UUID
    if (entityId && !isUUID(entityId) && !entityId.startsWith('#')) return true;

    // Keep if has other meaningful properties
    const excludeKeys = ['@id', '@type', 'name'];
    const otherProps = Object.keys(entity).filter(key => !excludeKeys.includes(key));
    return otherProps.some(key => {
      const value = entity[key];
      if (typeof value === 'string') return !isUUID(value);
      return true;
    });
  });

  if (meaningfulEntities.length === 0) {
    return '';
  }

  // Generate unique ID for accordion
  const accordionId = 'otherMetadataAccordion';
  const collapseId = 'otherMetadataCollapse';

  let entitiesHtml = '';
  meaningfulEntities.forEach(entity => {
    entitiesHtml += renderUnmappedEntity(entity);
  });

  return `
    <div class="accordion mb-3" id="${accordionId}">
      <div class="accordion-item">
        <h2 class="accordion-header">
          <button class="accordion-button collapsed fw-semibold bg-light" type="button" data-bs-toggle="collapse" data-bs-target="#${collapseId}" aria-expanded="false" aria-controls="${collapseId}">
            <i class="bi bi-three-dots me-2 text-secondary"></i>OTHER METADATA (${meaningfulEntities.length})
          </button>
        </h2>
        <div id="${collapseId}" class="accordion-collapse collapse" data-bs-parent="#${accordionId}">
          <div class="accordion-body">
            ${entitiesHtml}
          </div>
        </div>
      </div>
    </div>
  `;
}

// Export functions for use in other modules
if (typeof module !== 'undefined' && module.exports) {
  module.exports = {
    extractRecordData,
    renderData,
      /*
    extractOwner,
    extractTeam,
    extractTags,
    extractStartDate,
    extractMainText,
    extractFiles,
    extractLinks,
    collectUnmappedEntities,
    isUUID,
    renderCommonInfoBlock,
    renderMainTextBlock,
    renderMainTextSection,
    renderExtraFieldsBlock,
    renderAttachedFilesSubsection,
    renderLinksSubsection,
    renderCompoundsSubsection,
    renderStorageSubsection,
    renderPermissionsSubsection,
    renderOtherMetadata,
    renderCustomFields,
    renderSteps,
    renderUnmappedEntity,
    sanitizeHTML,
    formatDate,
    formatFileSize,
    filterUUIDs,
    isOnlyUUID,
    escapeHtmlForRenderer
    */
  };
}

// Also make available globally for browser use
if (typeof window !== 'undefined') {
  window.RecordExtractor = {
    extractRecordData,
    renderData,
      /*
    extractOwner,
    extractTeam,
    extractTags,
    extractStartDate,
    extractMainText,
    extractFiles,
    extractLinks,
    collectUnmappedEntities,
    isUUID,
    renderCommonInfoBlock,
    renderMainTextBlock,
    renderMainTextSection,
    renderExtraFieldsBlock,
    renderAttachedFilesSubsection,
    renderLinksSubsection,
    renderCompoundsSubsection,
    renderStorageSubsection,
    renderPermissionsSubsection,
    renderOtherMetadata,
    renderCustomFields,
    renderSteps,
    renderUnmappedEntity,
    sanitizeHTML,
    formatDate,
    formatFileSize,
    filterUUIDs,
    isOnlyUUID,
    escapeHtmlForRenderer
      */
  };
}
