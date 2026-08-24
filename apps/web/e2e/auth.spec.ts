import { test, expect } from '@playwright/test';
import { setupApiMocks, loginAs } from './fixtures/api-mock';
import { mockRegularUser, mockAdminUser } from './fixtures/mock-data';

test.describe('Authentication & Route Guard Flows', () => {
  test.describe.configure({ mode: 'serial' });

  test('login with regular user credentials redirects to /dashboard and stores token', async ({ page }) => {
    await setupApiMocks(page, { currentUser: null });

    await page.goto('/login');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('h1')).toContainText('Welcome Back');

    await page.fill('input#login-email', 'coder@example.com');
    await page.fill('input#login-password', 'securepassword123');
    await page.click('button[type="submit"]');

    await expect(page).toHaveURL('/dashboard');
    await expect(page.locator('nav').first()).toContainText('tourist_fan');

    const token = await page.evaluate(() => localStorage.getItem('cp_token'));
    expect(token).toBeTruthy();
  });

  test('login with admin credentials redirects to /admin dashboard', async ({ page }) => {
    await setupApiMocks(page, { currentUser: null });

    await page.goto('/login');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('h1')).toContainText('Welcome Back');

    await page.fill('input#login-email', 'admin@cpbridge.dev');
    await page.fill('input#login-password', 'adminpassword123');
    await page.click('button[type="submit"]');

    await expect(page).toHaveURL('/admin');
    await expect(page.locator('nav').first()).toContainText('root_admin');
    await expect(page.locator('nav').first()).toContainText('ADMIN');
  });

  test('login with invalid credentials shows an error message', async ({ page }) => {
    await setupApiMocks(page, { currentUser: null });

    await page.goto('/login');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('h1')).toContainText('Welcome Back');

    await page.fill('input#login-email', 'wrong@example.com');
    await page.fill('input#login-password', 'wrongpassword');
    await page.click('button[type="submit"]');

    await expect(page.locator('text=Invalid username or password')).toBeVisible();
    await expect(page).toHaveURL('/login');
  });

  test('registration form submits and registers a new account', async ({ page }) => {
    await setupApiMocks(page, { currentUser: null });

    await page.goto('/register');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('h1')).toContainText('Create Account');

    await page.fill('input#reg-username', 'alice_coder');
    await page.fill('input#reg-email', 'alice@example.com');
    await page.fill('input#reg-password', 'mySecret123');
    await page.click('button[type="submit"]');

    await expect(page).toHaveURL('/dashboard');
  });

  test('logout action clears stored token and returns navbar to guest mode', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, { currentUser: mockRegularUser });

    await page.goto('/dashboard');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('nav').first()).toContainText('tourist_fan');

    // Click logout button (with title 'Log Out')
    await page.click('button[title="Log Out"]');

    await expect(page.locator('nav').first()).toContainText('Sign In');
    await expect(page.locator('nav').first()).toContainText('Sign Up');

    const token = await page.evaluate(() => localStorage.getItem('cp_token'));
    expect(token).toBeNull();
  });

  test('RBAC guard: unauthenticated guest accessing /admin is redirected to /contests', async ({ page }) => {
    await setupApiMocks(page, { currentUser: null });

    await page.goto('/admin');
    await page.waitForLoadState('networkidle');

    await expect(page).toHaveURL('/contests');
    await expect(page.locator('h1')).toContainText('Contests');
  });

  test('RBAC guard: regular user accessing /admin is redirected to /contests', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, { currentUser: mockRegularUser });

    await page.goto('/admin');
    await page.waitForLoadState('networkidle');

    await expect(page).toHaveURL('/contests');
    await expect(page.locator('h1')).toContainText('Contests');
  });

  test('RBAC guard: admin user visiting /dashboard is automatically redirected to /admin', async ({ page }) => {
    await loginAs(page, mockAdminUser);
    await setupApiMocks(page, { currentUser: mockAdminUser });

    await page.goto('/dashboard');
    await page.waitForLoadState('networkidle');

    await expect(page).toHaveURL('/admin');
    await expect(page.locator('h1')).toContainText('Admin Dashboard');
  });
});
