export function formatUnixTimestamp(unixTimestampStr) {
  // Convert the Unix timestamp string into a Date object.
  const targetDate = new Date(parseInt(unixTimestampStr, 10) * 1000);
  const now = new Date();
  const diffInMs = targetDate - now;

  // Calculate the difference in seconds.
  const diffInSeconds = Math.round(diffInMs / 1000);

  // Decide which unit to use for a cleaner output.
  let unit = 'second';
  let diff = diffInSeconds;

  // Use minutes if the difference is at least 60 seconds.
  if (Math.abs(diffInSeconds) >= 60) {
    diff = Math.round(diffInSeconds / 60);
    unit = 'minute';
  }

  // Use hours if the difference is at least 60 minutes.
  if (Math.abs(diff) >= 60 && unit === 'minute') {
    diff = Math.round(diff / 60);
    unit = 'hour';
  }

  // Use days if the difference is at least 24 hours.
  if (Math.abs(diff) >= 24 && unit === 'hour') {
    diff = Math.round(diff / 24);
    unit = 'day';
  }

  // Create a RelativeTimeFormat formatter.
  const rtf = new Intl.RelativeTimeFormat('en', { numeric: 'auto' });
  return rtf.format(diff, unit);
}

export function formatSize(size) {
  return new Intl.NumberFormat('en', {
  style: 'unit',
  unit: 'megabyte',
  unitDisplay: 'short',
  maximumFractionDigits: 2,
  }).format(size / (1024 * 1024));
};

export function mkSpin(el) {
  // we want the button to keep the same size, so store width/height as style attribute
  const { width, height } = el.getBoundingClientRect();
  el.style.width = `${width}px`;
  el.style.height = `${height}px`;
  // keep the old html around so we can restore it
  const elOldHTML = el.innerHTML;
  el.setAttribute('disabled', 'disabled');
  el.innerHTML = '<span class="spinner"></span>';
  return elOldHTML;
}
