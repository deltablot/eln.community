(()=>{function l(e){let t=document.createElement("div");return t.textContent=e,t.innerHTML}function g(e){let t=new Date(e),n=new Date-t,o=Math.floor(n/6e4),i=Math.floor(n/36e5),r=Math.floor(n/864e5);return o<1?"just now":o<60?`${o} minute${o>1?"s":""} ago`:i<24?`${i} hour${i>1?"s":""} ago`:r<7?`${r} day${r>1?"s":""} ago`:t.toLocaleDateString("en-US",{year:"numeric",month:"short",day:"numeric",hour:"2-digit",minute:"2-digit"})}function p(e){let t=e.moderation_status==="pending_review"?'<span class="badge bg-warning text-dark ms-2">Pending Review</span>':e.moderation_status==="rejected"?'<span class="badge bg-danger ms-2">Rejected</span>':"";return`
    <div class="card mb-3 comment-item" data-comment-id="${e.id}">
      <div class="card-body">
        <div class="d-flex justify-content-between align-items-start mb-2">
          <div>
            <strong>
              <a href="https://orcid.org/${l(e.commenter_orcid)}" 
                 target="_blank" 
                 rel="noopener noreferrer" 
                 class="text-decoration-none">
                ${l(e.commenter_name)}
              </a>
            </strong>
            ${t}
          </div>
          <small class="text-muted">${g(e.created_at)}</small>
        </div>
        <p class="card-text" style="white-space: pre-wrap;">${l(e.content)}</p>
      </div>
    </div>
  `}async function u(e){let t=document.getElementById("comments-list"),s=document.getElementById("comment-count");try{let n=await fetch(`/api/v1/records/${e}/comments`);if(!n.ok){if(n.status===404){s.textContent="0",t.innerHTML=`
          <div class="text-center p-4 text-muted">
            <i class="bi bi-chat-left-text" style="font-size: 2rem;"></i>
            <p class="mt-2">No comments yet. Be the first to comment!</p>
          </div>
        `;return}throw new Error("Failed to load comments")}let o=await n.json(),i=o.filter(r=>r.moderation_status==="approved");if(s.textContent=i.length,o.length===0)t.innerHTML=`
        <div class="text-center p-4 text-muted">
          <i class="bi bi-chat-left-text" style="font-size: 2rem;"></i>
          <p class="mt-2">No comments yet. Be the first to comment!</p>
        </div>
      `;else{let r=o.map(p).join(""),c=o.filter(m=>m.moderation_status==="pending_review").length,a=o.filter(m=>m.moderation_status==="rejected").length,d="";if(c>0||a>0){let m=[];c>0&&m.push(`${c} pending review`),a>0&&m.push(`${a} rejected`),d=`
          <div class="alert alert-info alert-dismissible fade show" role="alert">
            <i class="bi bi-info-circle me-2"></i>
            <strong>Admin View:</strong> You are seeing all comments including ${m.join(" and ")}.
            <button type="button" class="btn-close" data-bs-dismiss="alert" aria-label="Close"></button>
          </div>
        `}t.innerHTML=d+r}}catch(n){console.error("Error loading comments:",n),t.innerHTML=`
      <div class="alert alert-danger">
        <i class="bi bi-exclamation-triangle me-2"></i>
        Failed to load comments. Please try again later.
      </div>
    `}}async function b(e,t){let s=await fetch(`/api/v1/records/${e}/comments`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({content:t})});if(!s.ok){let n=await s.text();throw new Error(n||"Failed to post comment")}return await s.json()}function f(){let e=document.getElementById("record-id-data");if(!e){console.error("Record ID not found");return}let t=JSON.parse(e.textContent);u(t);let s=document.getElementById("comment-form");if(s){let n=document.getElementById("comment-content"),o=document.getElementById("char-count"),i=document.getElementById("submit-comment-btn");n.addEventListener("input",()=>{let r=n.value.length;o.textContent=r,r>5e3?o.classList.add("text-danger"):o.classList.remove("text-danger")}),s.addEventListener("submit",async r=>{r.preventDefault();let c=n.value.trim();if(!c){alert("Please enter a comment");return}if(c.length>5e3){alert("Comment is too long (maximum 5000 characters)");return}i.disabled=!0,i.innerHTML='<span class="spinner-border spinner-border-sm me-1"></span>Posting...';try{let a=await b(t,c);n.value="",o.textContent="0";let d=document.createElement("div");d.className="alert alert-success alert-dismissible fade show",d.innerHTML=`
          <i class="bi bi-check-circle me-2"></i>
          ${a.moderation_status==="pending_review"?"Comment submitted and is pending moderation.":"Comment posted successfully!"}
          <button type="button" class="btn-close" data-bs-dismiss="alert"></button>
        `,s.parentElement.insertBefore(d,s),setTimeout(()=>{d.remove()},5e3),await u(t)}catch(a){console.error("Error posting comment:",a),alert("Failed to post comment: "+a.message)}finally{i.disabled=!1,i.innerHTML='<i class="bi bi-send me-1"></i>Post Comment'}})}}document.readyState==="loading"?document.addEventListener("DOMContentLoaded",f):f();})();
