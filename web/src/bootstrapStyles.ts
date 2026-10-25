import bootstrapLTR from 'bootstrap/dist/css/bootstrap.min.css?url';
import bootstrapRTL from 'bootstrap/dist/css/bootstrap.rtl.min.css?url';

export function setBootstrapDirection(direction: 'ltr' | 'rtl'): void {
  let link = document.getElementById('bootstrap-direction-css') as HTMLLinkElement | null;
  if (!link) {
    link = document.createElement('link');
    link.id = 'bootstrap-direction-css';
    link.rel = 'stylesheet';
    // Load Bootstrap before the application stylesheet so our design tokens and component rules win.
    document.head.insertBefore(link, document.head.firstChild);
  }
  link.href = direction === 'rtl' ? bootstrapRTL : bootstrapLTR;
}