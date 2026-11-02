import { test, expect } from '@playwright/test';

test('dev sign-in lands on /vehicles with the dispatch bar visible', async ({ page }) => {
  await page.goto('/login');
  await page.selectOption('select', 'manager');
  await page.click('button[type="submit"]');

  await expect(page).toHaveURL(/\/vehicles$/);
  // Regression test for the bug this exact suite would have caught: the
  // bar used to only appear after a manual page reload, because
  // `signedIn` wasn't reactive — see src/auth.jsx's doc comment.
  await expect(page.locator('header.dispatch-bar')).toBeVisible();
  await expect(page.getByRole('link', { name: 'Vehicles' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Deliveries' })).toBeVisible();
});

test('unauthenticated visit to /vehicles redirects to /login', async ({ page }) => {
  await page.goto('/vehicles');
  await expect(page).toHaveURL(/\/login$/);
});

test('sign out clears the session and the guard kicks back in', async ({ page }) => {
  await page.goto('/login');
  await page.selectOption('select', 'manager');
  await page.click('button[type="submit"]');
  await expect(page).toHaveURL(/\/vehicles$/);

  await page.getByRole('button', { name: 'sign out' }).click();
  await expect(page).toHaveURL(/\/login$/);

  await page.goto('/vehicles');
  await expect(page).toHaveURL(/\/login$/);
});

test('clicking Deliveries in the nav actually navigates there', async ({ page }) => {
  await page.goto('/login');
  await page.selectOption('select', 'manager');
  await page.click('button[type="submit"]');

  await page.getByRole('link', { name: 'Deliveries' }).click();
  await expect(page).toHaveURL(/\/deliveries$/);
  await expect(page.getByRole('heading', { name: /Deliveries/ })).toBeVisible();
});