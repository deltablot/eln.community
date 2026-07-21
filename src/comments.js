import { formatDateTime } from './record-extractor.js';
import { updateCount } from './index.js';

const COMMENT_MAX_LENGTH = 5000;

const ModerationStatus = {
  Pending: 0,
  Approved: 1,
  Rejected: 2,
  Deleted: 3,
  Flagged: 4,
};

const ModerationStatusLabel = {
  [ModerationStatus.Pending]: 'Pending',
  [ModerationStatus.Rejected]: 'Rejected',
  [ModerationStatus.Flagged]: 'Reported',
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
    && comment.moderation_status !== ModerationStatus.Approved
    && comment.moderation_status !== ModerationStatus.Deleted;
}

function canRejectComment(comment) {
  return state.isAdmin
    && comment.moderation_status !== ModerationStatus.Rejected
    && comment.moderation_status !== ModerationStatus.Deleted;
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

async function handleSubmit(event) {
  event.preventDefault();
  const commentInput = document.getElementById('comment-input');
  const content = commentInput.value.trim();
  const submitCommentBtn = document.getElementById('submit-comment-btn');
  const charCount = document.getElementById('char-count');

  if (!content) {
      alert('Please enter a comment');
      return;
  }

  submitCommentBtn.disabled = true;
  try {
    const newComment = await createComment(content);
    state.comments.push(newComment);
    commentInput.value = '';
    charCount.textContent = '0';
    commentInput.required = false;
    renderAllComments(state.comments);
  } catch (err) {
    console.log('failed to create comment:', err);
  } finally {
    submitCommentBtn.disabled = false;
  }
}

function bindCommentForm() {
  updateCount('comment-input', 'char-count', 'comment-max', COMMENT_MAX_LENGTH);
  const commentForm = document.getElementById('comment-form');
  if (!commentForm)
    return;
  commentForm.addEventListener('submit', handleSubmit);
}

async function updateCommentStatus(url, httpMethod, action) {
  const req = await fetch(url, {
      method: httpMethod,
  });
  if (!req.ok)
    throw new Error(`failed to ${action} comment`);
  return req.json();
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
    await updateCommentStatus(`/api/v1/moderation/comments/${commentId}/approve`, 'POST', 'approve');
    await loadComments();
    return;
  }

  if (rejectBtn) {
    await updateCommentStatus(`/api/v1/moderation/comments/${commentId}/reject`, 'POST', 'reject');
    await loadComments();
    return;
  }

  if (deleteBtn) {
    await updateCommentStatus(`/api/v1/records/${state.recordId}/comments/${commentId}`, 'DELETE', 'delete');
    state.comments = state.comments.filter((comment) => String(comment.id) !== commentId);
    renderAllComments(state.comments);
    return;
  }

  if (flagBtn) {
    await updateCommentStatus(`/api/v1/records/${state.recordId}/comments/${commentId}/flag`, 'POST', 'dflag');
    await loadComments();
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

function displayCommentStatus(commentItem, comment) {
  const commentStatus = commentItem.querySelector('.comment-status-badge');
  const label =  ModerationStatusLabel[comment.moderation_status];
  if (!label)
    return;

  commentStatus.classList.remove('bg-warning', 'bg-danger');
  commentStatus.textContent = label;
  commentStatus.classList.remove('d-none');

  if (comment.moderation_status === ModerationStatus.Flagged) {
    commentStatus.classList.add('reported-flag');
    return;
  }
  if (comment.moderation_status === ModerationStatus.Rejected) {
    commentStatus.classList.add('rejected-flag');
    return;
  }

  commentStatus.classList.add('pending-flag');
}

function displayCommentActions(commentItem, comment) {
  const actions = commentItem.querySelector('.comment-actions');
  const summary = actions.querySelector('summary');

  const approveBtn = commentItem.querySelector('.comment-approve-btn');
  const rejectBtn = commentItem.querySelector('.comment-reject-btn');
  const deleteBtn = commentItem.querySelector('.comment-delete-btn');
  const flagBtn = commentItem.querySelector('.comment-flag-btn');

  const showApprove = canApproveComment(comment);
  const showReject = canRejectComment(comment);
  const showFlag = canFlagComment(comment);
  const showDelete = canDeleteComment(comment);

  approveBtn.classList.toggle('d-none', !showApprove);
  rejectBtn.classList.toggle('d-none', !showReject);
  flagBtn.classList.toggle('d-none', !showFlag);
  deleteBtn.classList.toggle('d-none', !showDelete);

  const hasActions = showApprove || showReject || showFlag || showDelete;
  actions.classList.toggle('d-none', !hasActions);

  if (!hasActions)
    return;

  const showActions = state.isAdmin && comment.moderation_status != ModerationStatus.Pending;

  actions.open = !showActions;
  summary.classList.toggle('d-none', !showActions);
}

function renderComment(comment) {
  const commentTemplate = document.getElementById('comment-template');
  const clone = document.importNode(commentTemplate.content, true);
  const commentItem = clone.querySelector('.comment-item');
  const commentAuthor = commentItem.querySelector('.comment-author-link');
  const commentDate = commentItem.querySelector('.comment-created-at');
  const commentContent = commentItem.querySelector('.comment-content');
  commentItem.dataset.commentId = comment.id;
  displayCommentActions(commentItem, comment);
  displayCommentStatus(commentItem, comment);

  commentAuthor.textContent = comment.commenter_name;
  commentAuthor.href = "https://orcid.org/" + comment.commenter_orcid;
  commentDate.textContent = formatDateTime(comment.created_at);
  commentContent.textContent = comment.content;

  return commentItem;
}

function updateCommentTotal() {
  const commentCount = document.getElementById('comment-count');
  if (!commentCount)
      return;
  commentCount.textContent = state.comments.length;
}

function renderAllComments(comments) {
  const commentsList = document.getElementById('comments-list');
  commentsList.replaceChildren();
  comments.forEach((comment) => {
    const commentEl = renderComment(comment);
    commentsList.appendChild(commentEl);
  });
  updateCommentTotal();
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

// export { ModerationStatus };
// charger les commentaires
// créer un commentaire
// supprimer un commentaire
// flag un commentaire
// approve / reject côté admin
// afficher les commentaires
// décider quels boutons afficher
// gérer les erreurs
