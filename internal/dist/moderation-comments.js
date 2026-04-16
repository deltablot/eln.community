(()=>{function m(e){let t=document.createElement("div");return t.textContent=e,t.innerHTML}function g(e){return new Date(e).toLocaleDateString("en-US",{year:"numeric",month:"short",day:"numeric",hour:"2-digit",minute:"2-digit"})}function p(e){return`
    <div class="card mb-3" data-comment-id="${e.id}">
      <div class="card-body">
        <div class="d-flex justify-content-between align-items-start mb-3">
          <div class="flex-grow-1">
            <h6 class="card-subtitle mb-2">
              <a href="/record/${e.record_id}" class="text-decoration-none" target="_blank">
                View Record <i class="bi bi-box-arrow-up-right small"></i>
              </a>
            </h6>
            <div class="text-muted small mb-2">
              <strong>Commenter:</strong> 
              <a href="https://orcid.org/${m(e.commenter_orcid)}" 
                 target="_blank" 
                 rel="noopener noreferrer">
                ${m(e.commenter_name)}
              </a>
              (${m(e.commenter_orcid)})
            </div>
            <div class="text-muted small mb-2">
              <strong>Posted:</strong> ${g(e.created_at)}
            </div>
          </div>
        </div>

        <div class="mb-3">
          <div class="bg-light rounded p-3">
            <p class="mb-0" style="white-space: pre-wrap;">${m(e.content)}</p>
          </div>
        </div>

        <div class="d-flex gap-2">
          <button class="btn btn-success btn-sm approve-comment-btn" 
                  data-comment-id="${e.id}">
            <span class="spinner-border spinner-border-sm d-none me-1"></span>
            <i class="bi bi-check-circle me-1"></i>Approve
          </button>
          <button class="btn btn-danger btn-sm reject-comment-btn" 
                  data-comment-id="${e.id}">
            <span class="spinner-border spinner-border-sm d-none me-1"></span>
            <i class="bi bi-x-circle me-1"></i>Reject
          </button>
          <button class="btn btn-outline-danger btn-sm delete-comment-btn" 
                  data-comment-id="${e.id}">
            <span class="spinner-border spinner-border-sm d-none me-1"></span>
            <i class="bi bi-trash me-1"></i>Delete
          </button>
        </div>
      </div>
    </div>
  `}async function u(){let e=document.getElementById("pending-comments-container"),t=document.getElementById("pending-comments-count");try{let n=await fetch("/api/v1/moderation/comments?limit=50&offset=0");if(!n.ok)throw new Error("Failed to load pending comments");let r=await n.json(),o=r.comments||[];t.textContent=r.total||0,o.length===0?e.innerHTML=`
        <div class="alert alert-success text-center">
          <i class="bi bi-check-circle me-2"></i>
          No pending comments to review
        </div>
      `:(e.innerHTML=o.map(p).join(""),b())}catch(n){console.error("Error loading pending comments:",n),e.innerHTML=`
      <div class="alert alert-danger">
        <i class="bi bi-exclamation-triangle me-2"></i>
        Failed to load pending comments. Please try again later.
      </div>
    `}}function i(e,t){let n=e==="success"?"successToast":"errorToast",r=document.getElementById(n);e==="error"?document.getElementById("errorToastBody").textContent=t:r.querySelector(".toast-body").textContent=t,new bootstrap.Toast(r).show()}async function l(e,t){let n={approve:`/api/v1/moderation/comments/${e}/approve`,reject:`/api/v1/moderation/comments/${e}/reject`,delete:`/api/v1/moderation/comments/${e}`},r={approve:"POST",reject:"POST",delete:"DELETE"},o=await fetch(n[t],{method:r[t],headers:{"Content-Type":"application/json"},body:JSON.stringify({})});if(!o.ok){let c=await o.text();throw new Error(c||`Failed to ${t} comment`)}return await o.json()}function b(){document.querySelectorAll(".approve-comment-btn").forEach(e=>{e.addEventListener("click",async t=>{let n=t.currentTarget.dataset.commentId,r=t.currentTarget.querySelector(".spinner-border"),o=t.currentTarget.querySelector("i.bi-check-circle");t.currentTarget.disabled=!0,r.classList.remove("d-none"),o.classList.add("d-none");try{await l(n,"approve"),i("success","Comment approved successfully"),document.querySelector(`[data-comment-id="${n}"]`).remove();let s=document.getElementById("pending-comments-count"),d=parseInt(s.textContent);s.textContent=Math.max(0,d-1);let a=document.getElementById("pending-comments-container");a.querySelectorAll(".card").length===0&&(a.innerHTML=`
            <div class="alert alert-success text-center">
              <i class="bi bi-check-circle me-2"></i>
              No pending comments to review
            </div>
          `)}catch(c){console.error("Error approving comment:",c),i("error","Failed to approve comment: "+c.message),t.currentTarget.disabled=!1,r.classList.add("d-none"),o.classList.remove("d-none")}})}),document.querySelectorAll(".reject-comment-btn").forEach(e=>{e.addEventListener("click",async t=>{let n=t.currentTarget.dataset.commentId,r=t.currentTarget.querySelector(".spinner-border"),o=t.currentTarget.querySelector("i.bi-x-circle");if(confirm("Are you sure you want to reject this comment?")){t.currentTarget.disabled=!0,r.classList.remove("d-none"),o.classList.add("d-none");try{await l(n,"reject"),i("success","Comment rejected successfully"),document.querySelector(`[data-comment-id="${n}"]`).remove();let s=document.getElementById("pending-comments-count"),d=parseInt(s.textContent);s.textContent=Math.max(0,d-1);let a=document.getElementById("pending-comments-container");a.querySelectorAll(".card").length===0&&(a.innerHTML=`
            <div class="alert alert-success text-center">
              <i class="bi bi-check-circle me-2"></i>
              No pending comments to review
            </div>
          `)}catch(c){console.error("Error rejecting comment:",c),i("error","Failed to reject comment: "+c.message),t.currentTarget.disabled=!1,r.classList.add("d-none"),o.classList.remove("d-none")}}})}),document.querySelectorAll(".delete-comment-btn").forEach(e=>{e.addEventListener("click",async t=>{let n=t.currentTarget.dataset.commentId,r=t.currentTarget.querySelector(".spinner-border"),o=t.currentTarget.querySelector("i.bi-trash");if(confirm("Are you sure you want to permanently delete this comment? This action cannot be undone.")){t.currentTarget.disabled=!0,r.classList.remove("d-none"),o.classList.add("d-none");try{await l(n,"delete"),i("success","Comment deleted successfully"),document.querySelector(`[data-comment-id="${n}"]`).remove();let s=document.getElementById("pending-comments-count"),d=parseInt(s.textContent);s.textContent=Math.max(0,d-1);let a=document.getElementById("pending-comments-container");a.querySelectorAll(".card").length===0&&(a.innerHTML=`
            <div class="alert alert-success text-center">
              <i class="bi bi-check-circle me-2"></i>
              No pending comments to review
            </div>
          `)}catch(c){console.error("Error deleting comment:",c),i("error","Failed to delete comment: "+c.message),t.currentTarget.disabled=!1,r.classList.add("d-none"),o.classList.remove("d-none")}}})})}document.readyState==="loading"?document.addEventListener("DOMContentLoaded",u):u();})();
