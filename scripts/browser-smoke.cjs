// Run with the local demo server. npm install --no-save playwright, then node scripts/browser-smoke.cjs.
const fs = require("node:fs");
const screenshotDir = process.env.SCREENSHOT_DIR || "build/screenshots";
fs.mkdirSync(screenshotDir, { recursive: true });
if (!process.env.ADMIN_KEY) throw new Error("Set ADMIN_KEY to the local server key");
const { chromium } = require("playwright");
(async () => {
  const browser = await chromium.launch({
    ...(process.env.CHROME_PATH
      ? { executablePath: process.env.CHROME_PATH }
      : {}),
    headless: true,
    args: ["--no-sandbox"],
  });
  const page = await browser.newPage({
    viewport: { width: 1440, height: 1000 },
  });
  await page.goto("http://localhost:8080/?i=0123456789abcdef0123456789abcdef");
  await page.waitForSelector("#rsvp-form:not([hidden])");
  await page.screenshot({
    path: screenshotDir + "/desktop.png",
    fullPage: true,
  });
  await page.getByLabel("¡Sí, asistiremos!").check();
  await page.getByLabel("Adultos").fill("2");
  await page.getByLabel("Niños").fill("3");
  await page.getByRole("button", { name: "Guardar respuesta" }).click();
  await page.getByText("¡Confirmado! Los esperamos en la aventura.").waitFor();
  await page.goto("http://localhost:8080/admin.html");
  await page.locator("#key").fill(process.env.ADMIN_KEY);
  await page.getByRole("button", { name: "Entrar" }).click();
  await page.waitForSelector("#dashboard:not([hidden])");
  await page.screenshot({
    path: screenshotDir + "/admin.png",
    fullPage: true,
  });
  await page.goto("http://localhost:8080/send.html");
  await page.waitForSelector("#dashboard:not([hidden])");
  await page.locator("#device-name").fill("Test phone");
  await page.getByRole("button", { name: "Conectar teléfono" }).click();
  await page.locator("#pairing:not([hidden])").waitFor();
  const phone = await browser.newPage({
    viewport: { width: 390, height: 844 },
    isMobile: true,
  });
  await phone.goto("http://localhost:8080/?i=0123456789abcdef0123456789abcdef");
  await phone.waitForSelector("#rsvp-form:not([hidden])");
  await phone.screenshot({
    path: screenshotDir + "/mobile.png",
    fullPage: true,
  });
  if (
    await phone.evaluate(
      () => document.documentElement.scrollWidth > innerWidth,
    )
  )
    throw Error("Mobile horizontal overflow");
  console.log(
    "PASS: RSVP, admin login/report, device creation, mobile no overflow",
  );
  await browser.close();
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
