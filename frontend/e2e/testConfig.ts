const parsedPort = Number(process.env.PLAYWRIGHT_PORT ?? "4173");

if (!Number.isInteger(parsedPort) || parsedPort < 1 || parsedPort > 65535) {
  throw new Error("PLAYWRIGHT_PORT must be a valid TCP port");
}

export const browserTestPort = parsedPort;
export const appURL = `http://127.0.0.1:${browserTestPort}`;
