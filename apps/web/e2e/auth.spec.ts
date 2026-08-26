import { test, expect } from '@playwright/test';
import { setupApiMocks, loginAs } from './fixtures/api-mock';
import { mockRegularUser, mockAdminUser } from './fixtures/mock-data';

test.describe('Authentication & Route Guard Flows', () => {
  test.describe.configure({ mode: 'serial' });

  test('login with regular user credentials redirects to /contests and stores token', async ({ page }) => {
    await setupApiMocks(page, { currentUser: null });

    await page.goto('/login');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('h1')).toContainText('Welcome Back');

    await page.fill('input#login-email', 'coder@example.com');
    await page.fill('input#login-password', 'securepassword123');
    await page.click('button[type="submit"]');

    await expect(page).toHaveURL('/contests');
    await expect(page.locator('nav').first()).toContainText('tourist_fan');

    const token = await page.evaluate(() => localStorage.getItem('cp_token'));
    expect(token).toBeTruthy();
  });

  test('unauthenticated extension settings redirects to login and returns after sign in', async ({ page }) => {
    await setupApiMocks(page, { currentUser: null });

    await page.goto('/settings/integrations');
    await expect(page).toHaveURL('/login?returnTo=%2Fsettings%2Fintegrations');

    await page.fill('input#login-email', 'coder@example.com');
    await page.fill('input#login-password', 'securepassword123');
    await page.click('button[type="submit"]');

    await expect(page).toHaveURL('/settings/integrations');
    await expect(page.locator('h1')).toContainText('Platform Integrations');
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

    await expect(page).toHaveURL('/contests');
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

  test('expired JWT in localStorage is cleared on 401 and resets user to guest state', async ({ page }) => {
    // Inject invalid token but mock /auth/me to return 401
    await page.addInitScript(() => {
      localStorage.setItem('cp_token', 'expired_or_invalid_jwt');
    });
    await setupApiMocks(page, { currentUser: null });

    await page.goto('/contests');
    await page.waitForLoadState('networkidle');

    // Navbar should reflect guest mode
    await expect(page.locator('nav').first()).toContainText('Sign In');
    await expect(page.locator('nav').first()).toContainText('Sign Up');

    // cp_token must be removed
    const token = await page.evaluate(() => localStorage.getItem('cp_token'));
    expect(token).toBeNull();
  });

  test('page reload preserves authenticated session', async ({ page }) => {
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, { currentUser: mockRegularUser });

    await page.goto('/dashboard');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('nav').first()).toContainText('tourist_fan');

    // Reload page
    await page.reload();
    await page.waitForLoadState('networkidle');

    // Remains authenticated
    await expect(page.locator('nav').first()).toContainText('tourist_fan');
    await expect(page).toHaveURL('/dashboard');
  });

  test('registration form validates short passwords', async ({ page }) => {
    await setupApiMocks(page, { currentUser: null });

    await page.goto('/register');
    await page.waitForLoadState('networkidle');

    await page.fill('input#reg-username', 'short_pass_user');
    await page.fill('input#reg-email', 'short@example.com');
    await page.fill('input#reg-password', '123'); // < 6 chars
    await page.click('button[type="submit"]');

    await expect(page.locator('text=Password must be at least 6 characters')).toBeVisible();
  });

  test('root route / redirects to /contests for guests and /admin for admins', async ({ page }) => {
    // 1. Guest
    await setupApiMocks(page, { currentUser: null });
    await page.goto('/');
    await page.waitForURL('/contests');
    await expect(page.locator('h1')).toContainText('Contests');

    // 2. Regular user
    await loginAs(page, mockRegularUser);
    await setupApiMocks(page, { currentUser: mockRegularUser });
    await page.goto('/');
    await page.waitForURL('/contests');
    await expect(page.locator('h1')).toContainText('Contests');

    // 3. Admin user
    await loginAs(page, mockAdminUser);
    await setupApiMocks(page, { currentUser: mockAdminUser });
    await page.goto('/');
    await page.waitForURL('/admin');
    await expect(page.locator('h1')).toContainText('Admin Dashboard');
  });
});
