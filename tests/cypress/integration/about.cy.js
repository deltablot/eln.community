describe('about.html', () => {
  it('visit about', () => {
      cy.visit('/about')
      cy.url().should('include', '/about')
      cy.get('nav').should('exist')
      cy.contains('a', 'Sign In').should('have.attr', 'href')
      cy.get('h1').should('exist').contains('About')
      cy.get('footer').should('exist')
  })
})
