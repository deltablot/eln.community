describe('browse.html', () => {
  it('visit browse', () => {
      cy.visit('/browse')
      cy.url().should('include', '/browse')
      cy.get('nav').should('exist')
      cy.contains('a', 'Sign In').should('have.attr', 'href')
      cy.get('h2').should('exist').contains('Browse')
      cy.get('[class="category-multiselect-input"]').should('exist')
      cy.get('[class="browse-search-filter-container"]').should('exist')
      cy.contains('button', 'Search').should('exist')
      cy.get('footer').should('exist')
  })
})
