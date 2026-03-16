describe('organizations.html', () => {
  it('visit organizations', () => {
      cy.visit('/organizations')
      cy.url().should('include', '/organizations')
      cy.get('nav').should('exist')
      cy.contains('a', 'Sign In').should('have.attr', 'href')
      cy.get('h2').should('exist').contains('Organizations')
      cy.get('footer').should('exist')
  })
})
