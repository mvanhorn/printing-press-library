import { execFileRunner, type Runner } from "./process.js";

const LIBRARY_SOURCE = "mvanhorn/printing-press-library";

export interface InstallSkillOptions {
  agents?: string[];
  runner?: Runner;
}

export function skillSource(skillName: string): string {
  return `${LIBRARY_SOURCE}/cli-skills/${skillName}`;
}

export function skillsAddArgs(skillName: string, options: InstallSkillOptions = {}): string[] {
  const args = ["-y", "skills@latest", "add", skillSource(skillName), "-g", "-y"];
  for (const agent of options.agents ?? []) {
    args.push("-a", agent);
  }
  return args;
}

export async function installSkill(skillName: string, options: InstallSkillOptions = {}) {
  const runner = options.runner ?? execFileRunner;
  return runner("npx", skillsAddArgs(skillName, options));
}

export function skillsRemoveArgs(skillName: string, options: InstallSkillOptions = {}): string[] {
  const args = ["-y", "skills@latest", "remove", skillName, "-g", "-y"];
  for (const agent of options.agents ?? []) {
    args.push("-a", agent);
  }
  return args;
}

export async function removeSkill(skillName: string, options: InstallSkillOptions = {}) {
  const runner = options.runner ?? execFileRunner;
  return runner("npx", skillsRemoveArgs(skillName, options));
}
