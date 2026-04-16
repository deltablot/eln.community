/**
 * Comment moderation functionality for admin moderation page
 */

// Sanitize text content to prevent XSS
function sanitizeText(text) {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

// Format date for display
function formatDate(dateString) {
  const date = new Date(dateString);
  return date.toLocaleDateString('en-US', { 
    year: 'numeric', 
    month: 'short', 
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  });
}

// Render a pending comment card
function renderPendingComment(comment) {
  return `
    <div class="card mb-3" data-comment-id="${comment.id}">
      <div class="card-body">
        <div class="d-flex justify-content-between align-items-start mb-3">
          <div class="flex-grow-1">
            <h6 class="card-subtitle mb-2">
              <a href="/record/${comment.record_id}" class="text-decoration-none" target="_blank">
                View Record <i class="bi bi-box-arrow-up-right small"></i>
              </a>
            </h6>
            <div class="text-muted small mb-2">
              <strong>Commenter:</strong> 
              <a href="https://orcid.org/${sanitizeText(comment.commenter_orcid)}" 
                 target="_blank" 
                 rel="noopener noreferrer">
                ${sanitizeText(comment.commenter_name)}
              </a>
              (${sanitizeText(comment.commenter_orcid)})
            </div>
            <div class="text-muted small mb-2">
              <strong>Posted:</strong> ${formatDate(comment.created_at)}
            </div>
          </div>
        </div>

        <div class="mb-3">
          <div class="bg-light rounded p-3">
            <p class="mb-0" style="white-space: pre-wrap;">${sanitizeText(comment.content)}</p>
          </div>
        </div>

        <div class="d-flex gap-2">
          <button class="btn btn-success btn-sm approve-comment-btn" 
                  data-comment-id="${comment.id}">
            <span class="spinner-border spinner-border-sm d-none me-1"></span>
            <i class="bi bi-check-circle me-1"></i>Approve
          </button>
          <button class="btn btn-danger btn-sm reject-comment-btn" 
                  data-comment-id="${comment.id}">
            <span class="spinner-border spinner-border-sm d-none me-1"></span>
            <i class="bi bi-x-circle me-1"></i>Reject
          </button>
          <button class="btn btn-outline-danger btn-sm delete-comment-btn" 
                  data-comment-id="${comment.id}">
            <span class="spinner-border spinner-border-sm d-none me-1"></span>
            <i class="bi bi-trash me-1"></i>Delete
          </button>
        </div>
      </div>
    </div>
  `;
}

// Load pending comments
async function loadPendingComments() {
  const container = document.getElementById('pending-comments-container');
  const countBadge = document.getElementById('pending-comments-count');

  try {
    const response = await fetch('/api/v1/moderation/comments?limit=50&offset=0');
    
    if (!response.ok) {
      throw new Error('Failed to load pending comments');
    }

    const data = await response.json();
    const comments = data.comments || [];
    
    // Update count
    countBadge.textContent = data.total || 0;

    // Render comments
    if (comments.length === 0) {
      container.innerHTML = `
        <div class="alert alert-success text-center">
          <i class="bi bi-check-circle me-2"></i>
          No pending comments to review
        </div>
      `;
    } else {
      container.innerHTML = comments.map(renderPendingComment).join('');
      attachCommentEventHandlers();
    }
  } catch (error) {
    console.error('Error loading pending comments:', error);
    container.innerHTML = `
      <div class="alert alert-danger">
        <i class="bi bi-exclamation-triangle me-2"></i>
        Failed to load pending comments. Please try again later.
      </div>
    `;
  }
}

// Show toast notification
function showToast(type, message) {
  const toastId = type === 'success' ? 'successToast' : 'errorToast';
  const toastEl = document.getElementById(toastId);
  
  if (type === 'error') {
    document.getElementById('errorToastBody').textContent = message;
  } else {
    toastEl.querySelector('.toast-body').textContent = message;
  }
  
  const toast = new bootstrap.Toast(toastEl);
  toast.show();
}

// Moderate comment (approve/reject/delete)
async function moderateComment(commentId, action) {
  const endpoints = {
    approve: `/api/v1/moderation/comments/${commentId}/approve`,
    reject: `/api/v1/moderation/comments/${commentId}/reject`,
    delete: `/api/v1/moderation/comments/${commentId}`
  };

  const methods = {
    approve: 'POST',
    reject: 'POST',
    delete: 'DELETE'
  };

  const response = await fetch(endpoints[action], {
    method: methods[action],
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({})
  });

  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(errorText || `Failed to ${action} comment`);
  }

  return await response.json();
}

// Attach event handlers to comment action buttons
function attachCommentEventHandlers() {
  // Approve buttons
  document.querySelectorAll('.approve-comment-btn').forEach(btn => {
    btn.addEventListener('click', async (e) => {
      const commentId = e.currentTarget.dataset.commentId;
      const spinner = e.currentTarget.querySelector('.spinner-border');
      const icon = e.currentTarget.querySelector('i.bi-check-circle');
      
      e.currentTarget.disabled = true;
      spinner.classList.remove('d-none');
      icon.classList.add('d-none');

      try {
        await moderateComment(commentId, 'approve');
        showToast('success', 'Comment approved successfully');
        
        // Remove the comment card
        const card = document.querySelector(`[data-comment-id="${commentId}"]`);
        card.remove();
        
        // Update count
        const countBadge = document.getElementById('pending-comments-count');
        const currentCount = parseInt(countBadge.textContent);
        countBadge.textContent = Math.max(0, currentCount - 1);
        
        // Check if no more comments
        const container = document.getElementById('pending-comments-container');
        if (container.querySelectorAll('.card').length === 0) {
          container.innerHTML = `
            <div class="alert alert-success text-center">
              <i class="bi bi-check-circle me-2"></i>
              No pending comments to review
            </div>
          `;
        }
      } catch (error) {
        console.error('Error approving comment:', error);
        showToast('error', 'Failed to approve comment: ' + error.message);
        e.currentTarget.disabled = false;
        spinner.classList.add('d-none');
        icon.classList.remove('d-none');
      }
    });
  });

  // Reject buttons
  document.querySelectorAll('.reject-comment-btn').forEach(btn => {
    btn.addEventListener('click', async (e) => {
      const commentId = e.currentTarget.dataset.commentId;
      const spinner = e.currentTarget.querySelector('.spinner-border');
      const icon = e.currentTarget.querySelector('i.bi-x-circle');
      
      if (!confirm('Are you sure you want to reject this comment?')) {
        return;
      }

      e.currentTarget.disabled = true;
      spinner.classList.remove('d-none');
      icon.classList.add('d-none');

      try {
        await moderateComment(commentId, 'reject');
        showToast('success', 'Comment rejected successfully');
        
        // Remove the comment card
        const card = document.querySelector(`[data-comment-id="${commentId}"]`);
        card.remove();
        
        // Update count
        const countBadge = document.getElementById('pending-comments-count');
        const currentCount = parseInt(countBadge.textContent);
        countBadge.textContent = Math.max(0, currentCount - 1);
        
        // Check if no more comments
        const container = document.getElementById('pending-comments-container');
        if (container.querySelectorAll('.card').length === 0) {
          container.innerHTML = `
            <div class="alert alert-success text-center">
              <i class="bi bi-check-circle me-2"></i>
              No pending comments to review
            </div>
          `;
        }
      } catch (error) {
        console.error('Error rejecting comment:', error);
        showToast('error', 'Failed to reject comment: ' + error.message);
        e.currentTarget.disabled = false;
        spinner.classList.add('d-none');
        icon.classList.remove('d-none');
      }
    });
  });

  // Delete buttons
  document.querySelectorAll('.delete-comment-btn').forEach(btn => {
    btn.addEventListener('click', async (e) => {
      const commentId = e.currentTarget.dataset.commentId;
      const spinner = e.currentTarget.querySelector('.spinner-border');
      const icon = e.currentTarget.querySelector('i.bi-trash');
      
      if (!confirm('Are you sure you want to permanently delete this comment? This action cannot be undone.')) {
        return;
      }

      e.currentTarget.disabled = true;
      spinner.classList.remove('d-none');
      icon.classList.add('d-none');

      try {
        await moderateComment(commentId, 'delete');
        showToast('success', 'Comment deleted successfully');
        
        // Remove the comment card
        const card = document.querySelector(`[data-comment-id="${commentId}"]`);
        card.remove();
        
        // Update count
        const countBadge = document.getElementById('pending-comments-count');
        const currentCount = parseInt(countBadge.textContent);
        countBadge.textContent = Math.max(0, currentCount - 1);
        
        // Check if no more comments
        const container = document.getElementById('pending-comments-container');
        if (container.querySelectorAll('.card').length === 0) {
          container.innerHTML = `
            <div class="alert alert-success text-center">
              <i class="bi bi-check-circle me-2"></i>
              No pending comments to review
            </div>
          `;
        }
      } catch (error) {
        console.error('Error deleting comment:', error);
        showToast('error', 'Failed to delete comment: ' + error.message);
        e.currentTarget.disabled = false;
        spinner.classList.add('d-none');
        icon.classList.remove('d-none');
      }
    });
  });
}

// Initialize on page load
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', loadPendingComments);
} else {
  loadPendingComments();
}

export { loadPendingComments, moderateComment };
