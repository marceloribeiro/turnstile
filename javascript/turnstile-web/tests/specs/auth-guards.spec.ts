import { expect, test } from "@playwright/test";

// Unauthenticated visitors are redirected to /login, and the auth pages link
// to one another. These exercise client-side routing only (no seeded data).

test("the root route redirects an unauthenticated visitor to login", async ({ page }) => {
  await page.goto("/");
  await expect(page).toHaveURL(/\/login$/);
  await expect(page.getByRole("heading", { name: "Welcome back" })).toBeVisible();
});

test("a protected page redirects an unauthenticated visitor to login", async ({ page }) => {
  await page.goto("/dashboard");
  await expect(page).toHaveURL(/\/login$/);
});

test("the login and register pages link to each other", async ({ page }) => {
  await page.goto("/login");
  await page.getByRole("link", { name: "Create one" }).click();
  await expect(page).toHaveURL(/\/register$/);
  await expect(page.getByRole("heading", { name: "Create your account" })).toBeVisible();

  await page.getByRole("link", { name: "Sign in" }).click();
  await expect(page).toHaveURL(/\/login$/);
});

test("signing in with bad credentials shows an error and stays on /login", async ({ page }) => {
  await page.goto("/login");
  await page.getByPlaceholder("you@company.com").fill(`nobody-${Date.now()}@example.com`);
  await page.getByPlaceholder("••••••••").fill("wrong-password");
  await page.getByRole("button", { name: "Sign in" }).click();

  // The form surfaces the API error rather than navigating away.
  await expect(page).toHaveURL(/\/login$/);
  await expect(page.getByRole("button", { name: "Sign in" })).toBeEnabled();
});
