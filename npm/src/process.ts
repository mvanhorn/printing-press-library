import spawn from "cross-spawn";

export interface RunResult {
  code: number;
  stdout: string;
  stderr: string;
}

export interface RunOptions {
  env?: NodeJS.ProcessEnv;
}

export type Runner = (command: string, args: string[], options?: RunOptions) => Promise<RunResult>;

export const execFileRunner: Runner = (command, args, options = {}) => {
  return new Promise((resolve) => {
    const child = spawn(command, args, {
      env: options.env ? { ...process.env, ...options.env } : process.env,
    });
    let stdout = "";
    let stderr = "";
    child.stdout?.on("data", (chunk: Buffer) => {
      stdout += chunk.toString();
    });
    child.stderr?.on("data", (chunk: Buffer) => {
      stderr += chunk.toString();
    });
    child.on("error", (error: NodeJS.ErrnoException) => {
      if (error.code === "ENOENT") {
        resolve({ code: 127, stdout, stderr });
        return;
      }
      resolve({ code: 126, stdout, stderr });
    });
    child.on("close", (code) => {
      resolve({ code: code ?? 0, stdout, stderr });
    });
  });
};

export async function commandOnPath(
  binary: string,
  runner: Runner = execFileRunner,
  platform: NodeJS.Platform = process.platform,
): Promise<string | null> {
  const command = platform === "win32" ? "where" : "which";
  const result = await runner(command, [binary]);
  if (result.code !== 0) {
    return null;
  }
  return result.stdout.split(/\r?\n/).find((line) => line.trim() !== "")?.trim() ?? null;
}
