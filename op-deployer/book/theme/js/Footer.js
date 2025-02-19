// Create footer element
function createFooter() {
  const footer = document.createElement('footer');
  footer.className = 'mdbook-footer';

  const container = document.createElement('div');
  container.className = 'footer-container';

  // Add legal links
  const policyLinks = document.createElement('div');
  policyLinks.className = 'policy-links';

  const links = [
      { href: 'https://optimism.io/community-agreement', text: 'Community Agreement' },
      { href: 'https://optimism.io/terms', text: 'Terms of Service' },
      { href: 'https://optimism.io/data-privacy-policy', text: 'Privacy Policy' }
  ];

  links.forEach(link => {
      const a = document.createElement('a');
      a.href = link.href;
      a.textContent = link.text;
      policyLinks.appendChild(a);
  });

  // Add copyright notice
  const copyright = document.createElement('div');
  copyright.className = 'copyright';
  copyright.innerHTML = `© ${new Date().getFullYear()} <a href="https://optimism.io">Optimism Foundation. All rights reserved.</a>`;

  // Assemble footer
  container.appendChild(policyLinks);
  container.appendChild(copyright);
  footer.appendChild(container);

  // Add footer to page
  document.body.appendChild(footer);
}

// Run after DOM is loaded
document.addEventListener('DOMContentLoaded', createFooter);