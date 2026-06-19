import os
import subprocess
import hashlib
import base64

def main():
    print("[*] Starting custom AuraBlock Shield extension builder...")
    
    # 1. Create directories
    base_dir = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
    ext_dir = os.path.join(base_dir, "extension")
    os.makedirs(ext_dir, exist_ok=True)
    
    temp_dist_dir = os.path.join(base_dir, "extension_build")
    os.makedirs(temp_dist_dir, exist_ok=True)
    
    # Generate icon.png if it doesn't exist using base64 (no Pillow required!)
    if not os.path.exists(f"{ext_dir}/icon.png"):
        print("[*] Generating extension icon...")
        icon_base64 = """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"""
        with open(f"{ext_dir}/icon.png", "wb") as f:
            f.write(base64.b64decode(icon_base64))
    
    # 2. Generate Private Key if not exists
    pem_path = os.path.join(base_dir, "extension_key.pem")
    if not os.path.exists(pem_path):
        print("[*] Generating RSA private key for extension signing...")
        subprocess.run(f"openssl genrsa -out {pem_path} 2048", shell=True, check=True)
        
    # 3. Compute Extension ID
    print("[*] Calculating Extension ID...")
    der = subprocess.check_output(f"openssl rsa -in {pem_path} -pubout -outform DER 2>/dev/null", shell=True)
    sha256_hash = hashlib.sha256(der).hexdigest()
    
    # Translate sha256 hex digits to Chrome Extension ID hex representation (a-p)
    trans = str.maketrans('0123456789abcdef', 'abcdefghijklmnop')
    ext_id = sha256_hash[:32].translate(trans)
    print(f"[+] Computed Extension ID: {ext_id}")
    
    # 4. Pack Extension
    print("[*] Packaging extension into CRX file...")
    # Clean old build files if exist
    crx_path = os.path.join(base_dir, "extension.crx")
    if os.path.exists(crx_path):
        os.remove(crx_path)
        
    # Pack using Google Chrome CLI
    subprocess.run(f"google-chrome-stable --pack-extension={ext_dir} --pack-extension-key={pem_path} --no-sandbox", shell=True, check=True)
    
    # Move the crx file to build dir
    os.rename(crx_path, os.path.join(temp_dist_dir, "aurablock-shield.crx"))
    print("[+] Extension packed as aurablock-shield.crx")
    
    # 5. Write update.xml
    update_xml = f"""<?xml version='1.0' encoding='UTF-8'?>
<gupdate xmlns='http://www.google.com/update2/response' protocol='2.0'>
  <app appid='{ext_id}'>
    <updatecheck codebase='http://localhost:8082/extensions/aurablock-shield.crx' version='1.2' />
  </app>
</gupdate>"""
    with open(f"{temp_dist_dir}/update.xml", "w") as f:
        f.write(update_xml)
    print("[+] Generated update.xml manifest file.")
    
    # Output values for shell installer script
    with open(os.path.join(temp_dist_dir, "ext_id.txt"), "w") as f:
        f.write(ext_id)
        
    print("[+] Pre-build complete! Ready for installation script.")

if __name__ == "__main__":
    main()
