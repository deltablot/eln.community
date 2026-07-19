import { formatDateTime } from './record-extractor.js';

const ModerationStatus = {
  Pending: 0,
  Approved: 1,
  Rejected: 2,
  Deleted: 3,
  Flagged: 4,
};

const ModerationStatusLabel = {
  [ModerationStatus.Pending]: 'Pending',
  [ModerationStatus.Approved]: '',
  [ModerationStatus.Rejected]: 'Rejected',
  [ModerationStatus.Deleted]: 'Deleted',
  [ModerationStatus.Flagged]: 'Flagged',
};

const state = {
  recordId: null,
  currentUserOrcid: null,
  isAdmin: false,
  comments: [],
};

function readInitialState() {
    const commentsSection = document.getElementById('comments-section');

    state.recordId = commentsSection.dataset.recordId;
    state.currentUserOrcid = commentsSection.dataset.currentUserOrcid || null;
    state.isAdmin = commentsSection.dataset.currentUserIsAdmin === 'true';
}

function isAuthenticated() {
  return state.currentUserOrcid ? true : false;
}

function isCommentAuthor(comment) {
    return comment.commenter_orcid === state.currentUserOrcid;
}

function canApproveComment(comment) {
  return state.isAdmin
    && comment.moderation_status != ModerationStatus.Approved;
}

function canRejectComment(comment) {
  return state.isAdmin
    && comment.moderation_status != ModerationStatus.Rejected;
}

function canDeleteComment(comment) {
  return isAuthenticated() && (state.isAdmin || isCommentAuthor(comment));
}

function canFlagComment(comment) {
  return isAuthenticated()
    && !isCommentAuthor(comment)
    && comment.moderation_status === ModerationStatus.Approved;
}

async function fetchComments() {
  const res = await fetch(`/api/v1/records/${state.recordId}/comments`);
  if (!res.ok)
    throw new Error('failed to fetch comments');
  return res.json();
}

async function createComment(content) {
  const req = await fetch(`/api/v1/records/${state.recordId}/comments`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ content }),
  });
  if (!req.ok)
    throw new Error('failed to create comment');
  return req.json();
}

async function deleteComment(commentId) {
  const req = await fetch(`/api/v1/records/${state.recordId}/comments/${commentId}`, {
      method: 'DELETE',
  });
  if (!req.ok)
    throw new Error('failed to create comment');
  return req.json();
}

async function handleSubmit(event) {
  event.preventDefault();
  const commentInput = document.getElementById('comment-input');
  const content = commentInput.value.trim();
  const submitCommentBtn = document.getElementById('submit-comment-btn');

  if (!content) {
      alert('Please enter a comment');
      return;
  }

  submitCommentBtn.disabled = true;
  try {
    const newComment = await createComment(content);
    state.comments.push(newComment);
    renderAllComments(state.comments);
  } catch (err) {
    console.log('failed to create comment:', err);
  } finally {
    submitCommentBtn.disabled = false;
  }
}

function bindCommentForm() {
  const commentForm = document.getElementById('comment-form');
  if (!commentForm)
    return;
  commentForm.addEventListener('submit', handleSubmit);
}

async function handleStatus(event) {
  const commentItem = event.target.closest('.comment-item');
  if (!commentItem)
    return;
  const commentId = commentItem.dataset.commentId;
  const approveBtn = event.target.closest('.comment-approve-btn');
  const rejectBtn = event.target.closest('.comment-reject-btn');
  const deleteBtn = event.target.closest('.comment-delete-btn');
  const flagBtn = event.target.closest('.comment-flag-btn');

  if (approveBtn) {
    console.log('approve comment', commentId);
    return;
  }

  if (rejectBtn) {
    console.log('reject comment', commentId);
    return;
  }

  if (deleteBtn) {
    await deleteComment(commentId);
    state.comments = state.comments.filter((comment) => String(comment.id) !== commentId);
    renderAllComments(state.comments);
    console.log('delete comment', commentId);
    return;
  }

  if (flagBtn) {
    console.log('flag comment', commentId);
    return;
  }
}

function bindStatus() {
  const commentsList = document.getElementById('comments-list');
  if (!commentsList)
    return;
 commentsList.addEventListener('click', handleStatus);
}

function displayCommentActions(commentItem, comment) {
  const approveBtn = commentItem.querySelector('.comment-approve-btn');
  if (approveBtn)
    approveBtn.classList.toggle('d-none', !canApproveComment(comment));
  const rejectBtn = commentItem.querySelector('.comment-reject-btn');
  if (rejectBtn)
    rejectBtn.classList.toggle('d-none', !canRejectComment(comment));
  const deleteBtn = commentItem.querySelector('.comment-delete-btn');
  if (deleteBtn)
    deleteBtn.classList.toggle('d-none', !canDeleteComment(comment));
  const flagBtn = commentItem.querySelector('.comment-flag-btn');
  if (flagBtn)
    flagBtn.classList.toggle('d-none', !canFlagComment(comment));
}

function renderComment(comment) {
  const commentTemplate = document.getElementById('comment-template');
  const clone = document.importNode(commentTemplate.content, true);
  const commentItem = clone.querySelector('.comment-item');
  const commentAuthor = commentItem.querySelector('.comment-author-link');
  const commentStatus = commentItem.querySelector('.comment-status-badge');
  const commentDate = commentItem.querySelector('.comment-created-at');
  const commentContent = commentItem.querySelector('.comment-content');
  commentItem.dataset.commentId = comment.id;
  displayCommentActions(commentItem, comment);

  commentAuthor.textContent = comment.commenter_name;
  commentAuthor.href = "https://orcid.org/" + comment.commenter_orcid;
  commentStatus.textContent = comment.moderation_status;
  commentDate.textContent = formatDateTime(comment.created_at);
  commentContent.textContent = comment.content;

  return commentItem;
}

function renderAllComments(comments) {
  const commentsList = document.getElementById('comments-list');
  commentsList.replaceChildren();
  comments.forEach((comment) => {
    const commentEl = renderComment(comment);
    commentsList.appendChild(commentEl);
  });
}

async function loadComments() {
  try {
    const comments = await fetchComments();
    state.comments = comments;
    renderAllComments(state.comments);
  } catch (err) {
    console.error('Error loading comments:', err);
  }
}

function initComments() {
  readInitialState();
  bindCommentForm();
  loadComments();
  bindStatus();
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', initComments);
} else {
  initComments();
}

//export { loadComments, };

// export { loadComments, postComment, sanitizeText, };

// charger les commentaires
// créer un commentaire
// supprimer un commentaire
// flag un commentaire
// approve / reject côté admin
// afficher les commentaires
// décider quels boutons afficher
// gérer les erreurs

/*
// Sanitize text content to prevent XSS
function sanitizeText(text) {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

// Render a single comment
function renderComment(comment) {
  const statusBadge = comment.moderation_status === ModerationStatus.Pending
    ? '<span class="badge bg-warning text-dark ms-2">Pending</span>'
    : comment.moderation_status === ModerationStatus.Rejected
    ? '<span class="badge bg-danger ms-2">Rejected</span>'
    : '';

  return `
    <div class="card mb-3 comment-item" data-comment-id="${comment.id}">
      <div class="card-body">
        <div class="d-flex justify-content-between align-items-start mb-2">
          <div>
            <strong>
              <a href="https://orcid.org/${sanitizeText(comment.commenter_orcid)}"
                 target="_blank"
                 rel="noopener noreferrer"
                 class="text-decoration-none">
                ${sanitizeText(comment.commenter_name)}
              </a>
            </strong>
            ${statusBadge}
          </div>
          <small class="text-muted">${formatDateTime(comment.created_at)}</small>
        </div>
        ${ if comment.orcid }
        <p class="card-text" style="white-space: pre-wrap;">${sanitizeText(comment.content)}</p>
          <button class="btn btn-outline-danger btn-sm delete-comment-btn"
             data-comment-id="${comment.id}">
            <span class="spinner-border spinner-border-sm d-none"></span>
            <i class="bi bi-trash"></i>Delete
          </button>
          <!-- si tu n'es pas l'auteur, flag btn -->
          <button class="btn btn-outline-danger btn-sm delete-comment-btn"
             data-comment-id="${comment.id}">
            <span class="spinner-border spinner-border-sm d-none"></span>
            <i class="bi bi-trash"></i>Report
          </button>
      </div>
    </div>
  `;
}

// Load comments for the current record
async function loadComments(recordId) {
  const commentsList = document.getElementById('comments-list');
  const commentCount = document.getElementById('comment-count');

  try {
    const response = await fetch(`/api/v1/records/${recordId}/comments`);

    if (!response.ok) {
      // If 404, treat as no comments (endpoint might not exist yet)
      if (response.status === 404) {
        commentCount.textContent = '0';
        commentsList.innerHTML = `
          <div class="text-center p-4 text-muted">
            <i class="bi bi-chat-left-text" style="font-size: 2rem;"></i>
            <p class="mt-2">No comments yet. Be the first to comment!</p>
          </div>
        `;
        return;
      }
      throw new Error('Failed to load comments');
    }

    const comments = await response.json();

    // Update count - only count approved comments for public display
    const approvedComments = comments.filter(c => c.moderation_status === ModerationStatus.Approved);
    commentCount.textContent = approvedComments.length;

    // Render comments
    if (comments.length === 0) {
      commentsList.innerHTML = `
        <div class="text-center p-4 text-muted">
          <i class="bi bi-chat-left-text" style="font-size: 2rem;"></i>
          <p class="mt-2">No comments yet. Be the first to comment!</p>
        </div>
      `;
    } else {
      const commentsHtml = comments.map(renderComment).join('');

      // Show admin notice if there are pending/rejected comments
      const pendingCount = comments.filter(c => c.moderation_status === ModerationStatus.Pending).length;
      const rejectedCount = comments.filter(c => c.moderation_status === ModerationStatus.Rejected).length;
      const flaggedCount = comments.filter(c => c.moderation_status === ModerationStatus.Flagged).length;

      let adminNotice = '';
      if (pendingCount > 0 || rejectedCount > 0 || flaggedCount > 0) {
        const notices = [];
        if (pendingCount > 0) notices.push(`${pendingCount} pending`);
        if (rejectedCount > 0) notices.push(`${rejectedCount} rejected`);
        if (flaggedCount > 0) notices.push(`${flaggedCount} flagged`);

        adminNotice = `
          <div class="alert alert-info alert-dismissible fade show" role="alert">
            <i class="bi bi-info-circle me-2"></i>
            <strong>Admin View:</strong> You are seeing all comments including ${notices.join(' and ')}.
            <button type="button" class="btn-close" data-bs-dismiss="alert" aria-label="Close"></button>
          </div>
        `;
      }

      commentsList.innerHTML = adminNotice + commentsHtml;
    }
  } catch (error) {
    console.error('Error loading comments:', error);
    commentsList.innerHTML = `
      <div class="alert alert-danger">
        <i class="bi bi-exclamation-triangle me-2"></i>
        Failed to load comments. Please try again later, THANKS.
      </div>
    `;
  }
}

// Post a new comment
async function postComment(recordId, content) {
  const response = await fetch(`/api/v1/records/${recordId}/comments`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ content }),
  });

  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(errorText || 'Failed to post comment');
  }

  return await response.json();
}

// Initialize comments functionality
function initComments() {
  const recordIdElement = document.getElementById('record-id-data');
  if (!recordIdElement) {
    console.error('Record ID not found');
    return;
  }

  const recordId = JSON.parse(recordIdElement.textContent);

  // Load comments on page load
  loadComments(recordId);

  // Handle comment form submission
  const commentForm = document.getElementById('comment-form');
  if (commentForm) {
    const contentTextarea = document.getElementById('comment-content');
    const charCount = document.getElementById('char-count');
    const submitBtn = document.getElementById('submit-comment-btn');

    // Update character count
    contentTextarea.addEventListener('input', () => {
      const length = contentTextarea.value.length;
      charCount.textContent = length;

      if (length > 5000) {
        charCount.classList.add('text-danger');
      } else {
        charCount.classList.remove('text-danger');
      }
    });

    // Handle form submission
    commentForm.addEventListener('submit', async (e) => {
      e.preventDefault();

      const content = contentTextarea.value.trim();
      if (!content) {
        alert('Please enter a comment');
        return;
      }

      if (content.length > 5000) {
        alert('Comment is too long (maximum 5000 characters)');
        return;
      }

      // Disable form during submission
      submitBtn.disabled = true;
      submitBtn.innerHTML = '<span class="spinner-border spinner-border-sm me-1"></span>Posting...';

      try {
        const comment = await postComment(recordId, content);
        // Clear form
        contentTextarea.value = '';
        charCount.textContent = '0';

        // Show success message
        const successAlert = document.createElement('div');
        successAlert.className = 'alert alert-success alert-dismissible fade show';
        successAlert.innerHTML = `
          <i class="bi bi-check-circle me-2"></i>
          ${comment.moderation_status === ModerationStatus.Pending
            ? 'Comment submitted and is pending moderation.'
            : 'Comment posted successfully!'}
          <button type="button" class="btn-close" data-bs-dismiss="alert"></button>
        `;
        commentForm.parentElement.insertBefore(successAlert, commentForm);

        // Auto-dismiss after 5 seconds
        setTimeout(() => {
          successAlert.remove();
        }, 5000);

        // Reload comments
        await loadComments(recordId);

      } catch (error) {
        console.error('Error posting comment:', error);
        alert('Failed to post comment: ' + error.message);
      } finally {
        // Re-enable form
        submitBtn.disabled = false;
        submitBtn.innerHTML = '<i class="bi bi-send me-1"></i>Post Comment';
      }
    });
  }
}

// Initialize when DOM is ready
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', initComments);
} else {
  initComments();
}

// Export for use in other modules
export { loadComments, postComment, sanitizeText, };
*/
