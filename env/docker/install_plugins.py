import os
import subprocess

BASE_DIR = "/home/mindfulbytes/src/plugins/"
PLUGIN_DEST_BASE_DIR = "/home/mindfulbytes/bin/plugins"

def run_install_script(script, name, directory):
    plugin_dest = PLUGIN_DEST_BASE_DIR + os.path.sep + name
    
    print("Running %s in %s" %(script, directory))
    
    new_env = os.environ.copy()
    new_env["PLUGIN_DEST"] = plugin_dest

    os.mkdir(plugin_dest)
    
    subprocess.check_call(["chmod", "u+rx", script], cwd=directory, env=new_env)

    subprocess.check_call([script], cwd=directory, env=new_env)
    

if __name__ == "__main__":
    folders = os.listdir(BASE_DIR)
    for folder in folders:
        plugin_path = os.path.join(BASE_DIR, folder)
        if os.path.isdir(plugin_path):
            if os.path.exists(plugin_path + os.path.sep + "install.sh"):
                run_install_script("./install.sh", folder, plugin_path)
            elif os.path.exists(plugin_path + os.path.sep + "install.py"):
                run_install_script("python3 install.py", folder, plugin_path)
            else:
                print("no install script found in folder %s" %plugin_path)
    
