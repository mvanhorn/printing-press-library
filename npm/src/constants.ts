export const CLI_COMMAND_NAME = "printing-press-library";
export const NPM_PACKAGE_NAME = "@mvanhorn/printing-press-library";
export const NPX_COMMAND_PREFIX = `npx -y ${NPM_PACKAGE_NAME}`;

export function commandPrefixForInvocation(
  scriptPath = process.argv[1] ?? "",
  env: NodeJS.ProcessEnv = process.env,
): string {
  if (isNpxInvocation(scriptPath, env)) {
    return NPX_COMMAND_PREFIX;
  }
  return CLI_COMMAND_NAME;
}

function isNpxInvocation(scriptPath: string, env: NodeJS.ProcessEnv): boolean {
  if (env.npm_command === "exec") {
    return true;
  }
  return /(^|[/\\])_npx([/\\]|$)/.test(scriptPath);
}
