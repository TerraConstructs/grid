/**
 * Keycloak management helpers for E2E tests
 *
 * These helpers manage Keycloak user groups via docker exec commands
 * and trigger IAM cache refresh in gridapi.
 */

import { exec } from 'child_process';
import { promisify } from 'util';
import * as fs from 'fs/promises';

const execAsync = promisify(exec);

const GRIDAPI_PID_FILE = '/tmp/grid-e2e-gridapi.pid';

/**
 * Execute kcadm command inside Keycloak container
 *
 * @param args kcadm command arguments
 * @returns Command output
 */
async function kcadm(...args: string[]): Promise<string> {
  const command = `docker compose exec -T keycloak /opt/keycloak/bin/kcadm.sh ${args.join(' ')}`;
  const { stdout, stderr } = await execAsync(command);

  if (stderr && !stderr.includes('Logging into')) {
    console.warn('[kcadm] stderr:', stderr);
  }

  return stdout.trim();
}

/**
 * Add a user to a Keycloak group
 *
 * @param email User email (e.g., alice@example.com)
 * @param groupName Group name (e.g., platform-engineers)
 */
export async function addUserToGroup(
  email: string,
  groupName: string
): Promise<void> {
  console.log(`[Keycloak] Adding ${email} to group ${groupName}`);

  // Get user ID
  const userId = await kcadm(
    'get', 'users',
    '-r', 'grid',
    '-q', `username=${email}`,
    '--fields', 'id',
    '--format', 'csv',
    '--noquotes'
  );

  if (!userId) {
    throw new Error(`User ${email} not found in Keycloak`);
  }

  const userIdClean = userId.split('\n').filter(line =>
    line && !line.startsWith('id')
  )[0];

  // Get group ID
  const groupData = await kcadm(
    'get', 'groups',
    '-r', 'grid',
    '--fields', 'id,name',
    '--format', 'csv',
    '--noquotes'
  );

  const groupLine = groupData.split('\n').find(line =>
    line.includes(groupName)
  );

  if (!groupLine) {
    throw new Error(`Group ${groupName} not found in Keycloak`);
  }

  const groupId = groupLine.split(',')[0];

  // Add user to group
  try {
    await kcadm(
      'update', `users/${userIdClean}/groups/${groupId}`,
      '-r', 'grid',
      '-n'
    );
    console.log(`[Keycloak] ✓ ${email} added to ${groupName}`);
  } catch (error) {
    // Ignore error if user already in group
    if (error instanceof Error && !error.message.includes('409')) {
      throw error;
    }
    console.log(`[Keycloak] ${email} already in ${groupName}`);
  }
}

/**
 * Remove a user from a Keycloak group
 *
 * @param email User email (e.g., alice@example.com)
 * @param groupName Group name (e.g., platform-engineers)
 */
export async function removeUserFromGroup(
  email: string,
  groupName: string
): Promise<void> {
  console.log(`[Keycloak] Removing ${email} from group ${groupName}`);

  // Get user ID
  const userId = await kcadm(
    'get', 'users',
    '-r', 'grid',
    '-q', `username=${email}`,
    '--fields', 'id',
    '--format', 'csv',
    '--noquotes'
  );

  if (!userId) {
    console.warn(`[Keycloak] User ${email} not found`);
    return;
  }

  const userIdClean = userId.split('\n').filter(line =>
    line && !line.startsWith('id')
  )[0];

  // Get group ID
  const groupData = await kcadm(
    'get', 'groups',
    '-r', 'grid',
    '--fields', 'id,name',
    '--format', 'csv',
    '--noquotes'
  );

  const groupLine = groupData.split('\n').find(line =>
    line.includes(groupName)
  );

  if (!groupLine) {
    console.warn(`[Keycloak] Group ${groupName} not found`);
    return;
  }

  const groupId = groupLine.split(',')[0];

  // Remove user from group
  try {
    await kcadm(
      'delete', `users/${userIdClean}/groups/${groupId}`,
      '-r', 'grid'
    );
    console.log(`[Keycloak] ✓ ${email} removed from ${groupName}`);
  } catch (error) {
    console.warn(`[Keycloak] Failed to remove ${email} from ${groupName}:`, error);
  }
}

/**
 * Trigger IAM cache refresh in gridapi via SIGHUP
 *
 * Sends SIGHUP signal to gridapi process to immediately refresh
 * the group-to-role cache, making permission changes take effect.
 */
export async function refreshIAMCache(): Promise<void> {
  try {
    // Read gridapi PID from file
    const pidContent = await fs.readFile(GRIDAPI_PID_FILE, 'utf-8');
    const pid = pidContent.trim();

    if (!pid) {
      throw new Error('gridapi PID file is empty');
    }

    console.log(`[IAM Cache] Sending SIGHUP to gridapi (PID: ${pid})`);

    // Send SIGHUP signal
    await execAsync(`kill -HUP ${pid}`);

    // Wait a moment for cache refresh to complete
    await new Promise(resolve => setTimeout(resolve, 1000));

    console.log('[IAM Cache] ✓ Cache refresh triggered');
  } catch (error) {
    throw new Error(`Failed to refresh IAM cache: ${error}`);
  }
}

/**
 * Add user to group and refresh IAM cache
 *
 * Convenience function that combines group addition with cache refresh.
 *
 * @param email User email
 * @param groupName Group name
 */
export async function addUserToGroupAndRefreshCache(
  email: string,
  groupName: string
): Promise<void> {
  await addUserToGroup(email, groupName);
  await refreshIAMCache();
}

/**
 * Remove user from group and refresh IAM cache
 *
 * Convenience function that combines group removal with cache refresh.
 *
 * @param email User email
 * @param groupName Group name
 */
export async function removeUserFromGroupAndRefreshCache(
  email: string,
  groupName: string
): Promise<void> {
  await removeUserFromGroup(email, groupName);
  await refreshIAMCache();
}
