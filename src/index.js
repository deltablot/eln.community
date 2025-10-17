document.addEventListener('DOMContentLoaded', function () {

  const errorDialog = document.getElementById('error-dialog');
  const closeButton = errorDialog.querySelector("button");
  closeButton.addEventListener("click", () => {
    errorDialog.close();
  });

  const tosLink = document.getElementById('tos-link');
  const tosDialog = document.getElementById('tos');
  tosLink.addEventListener("click", () => {
    tosDialog.showModal();
  });
  const closeButtonTos = tosDialog.querySelector("button");
  closeButtonTos.addEventListener("click", () => {
    tosDialog.close();
  });

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
          // Show success toast
          const successToast = document.getElementById('successToast');
          if (successToast) {
            const toast = new bootstrap.Toast(successToast, {
              delay: 3000,
              autohide: true
            });

            // Redirect to browse page after toast hides
            successToast.addEventListener('hidden.bs.toast', function () {
              window.location.href = '/browse';
            }, { once: true });

            toast.show();
          }

          this.reset();
        } else {
          // Show error toast
          const errorText = await response.text();
          const errorToast = document.getElementById('errorToast');
          const errorToastBody = document.getElementById('errorToastBody');
          if (errorToast && errorToastBody) {
            errorToastBody.textContent = errorText || 'Upload failed. Please try again.';
            const toast = new bootstrap.Toast(errorToast, {
              delay: 5000,
              autohide: true
            });
            toast.show();
          }
          console.error('Upload failed:', errorText);
        }
      } catch (error) {
        // Show error toast for network/other errors
        const errorToast = document.getElementById('errorToast');
        const errorToastBody = document.getElementById('errorToastBody');
        if (errorToast && errorToastBody) {
          errorToastBody.textContent = 'Network error. Please check your connection and try again.';
          const toast = new bootstrap.Toast(errorToast, {
            delay: 5000,
            autohide: true
          });
          toast.show();
        }
        console.error('Upload error:', error);
      } finally {
        // Reset button state
        if (submitBtn) {
          submitBtn.disabled = false;
          spinner?.classList.add('d-none');
          if (btnText) btnText.textContent = 'Send';
        }
      }
    });
  }

});
