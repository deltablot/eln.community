/**
 * Comments functionality for record pages
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
  const now = new Date();
  const diffMs = now - date;
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMs / 3600000);
  const diffDays = Math.floor(diffMs / 86400000);

  if (diffMins < 1) return 'just now';
  if (diffMins < 60) return `${diffMins} minute${diffMins > 1 ? 's' : ''} ago`;
  if (diffHours < 24) return `${diffHours} hour${diffHours > 1 ? 's' : ''} ago`;
  if (diffDays < 7) return `${diffDays} day${diffDays > 1 ? 's' : ''} ago`;
  
  return date.toLocaleDateString('en-US', { 
    year: 'numeric', 
    month: 'short', 
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  });
}

// Render a single comment
function renderComment(comment) {
  const statusBadge = comment.moderation_status === 'pending_review' 
    ? '<span class="badge bg-warning text-dark ms-2">Pending Review</span>'
    : comment.moderation_status === 'rejected'
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
          <small class="text-muted">${formatDate(comment.created_at)}</small>
        </div>
        <p class="card-text" style="white-space: pre-wrap;">${sanitizeText(comment.content)}</p>
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
      throw new Error('Failed to load comments');
    }

    const comments = await response.json();
    
    // Update count
    commentCount.textContent = comments.length;

    // Render comments
    if (comments.length === 0) {
      commentsList.innerHTML = `
        <div class="text-center p-4 text-muted">
          <i class="bi bi-chat-left-text" style="font-size: 2rem;"></i>
          <p class="mt-2">No comments yet. Be the first to comment!</p>
        </div>
      `;
    } else {
      commentsList.innerHTML = comments.map(renderComment).join('');
    }
  } catch (error) {
    console.error('Error loading comments:', error);
    commentsList.innerHTML = `
      <div class="alert alert-danger">
        <i class="bi bi-exclamation-triangle me-2"></i>
        Failed to load comments. Please try again later.
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
          ${comment.moderation_status === 'pending_review' 
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
export { loadComments, postComment, sanitizeText, formatDate };
