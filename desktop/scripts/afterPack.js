const { execSync } = require('child_process');
const path = require('path');

exports.default = async function (context) {
  if (context.electronPlatformName === 'darwin') {
    const appPath = path.join(context.appOutDir, `${context.packager.appInfo.productFilename}.app`);
    console.log(`[afterPack] Ad-hoc codesigning and un-quarantining: ${appPath}`);
    try {
      execSync(`xattr -cr "${appPath}"`);
      execSync(`codesign --force --deep --sign - "${appPath}"`);
      console.log(`[afterPack] Successfully signed ${appPath}`);
    } catch (err) {
      console.error(`[afterPack] Failed to sign:`, err);
    }
  }
};
