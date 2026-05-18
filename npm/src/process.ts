import { execFile } from "node:child_process";

export interface RunResult {
  code: number;
  stdout: string;
  stderr: string;
}

export interface RunOptions {
  env?: NodeJS.ProcessEnv;
  shell?: boolean;
}

export type Runner = (command: string, args: string[], options?: RunOptions) => Promise<RunResult>;

export const execFileRunner: Runner = (command, args, options = {}) => {
  return new Promise((resolve) => {
    execFile(
      command,
      args,
      {
        env: options.env ? { ...process.env, ...options.env } : process.env,
        shell: options.shell ?? false,
      },
      (error, stdout, stderr) => {
        if (error && "code" in error && error.code === "ENOENT") {
          resolve({ code: 127, stdout, stderr });
          return;
        }

        resolve({
          code: typeof error?.code === "number" ? error.code : 0,
          stdout,
          stderr,
        });
      },
    );
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
