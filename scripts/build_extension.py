import os
import subprocess
import hashlib
import base64

def main():
    print("[*] Starting custom AuraBlock Shield extension builder...")
    
    # 1. Create directories
    ext_dir = "/home/cyber/CODES/aurablock/extension"
    os.makedirs(ext_dir, exist_ok=True)
    
    dist_ext_dir = "/etc/aurablock/dist/extensions"
    # Note: Writing to /etc requires sudo, so we will generate everything in /tmp or extension dir first,
    # and then copy it using sudo in the shell script.
    temp_dist_dir = "/home/cyber/CODES/aurablock/extension_build"
    os.makedirs(temp_dist_dir, exist_ok=True)
    
    # Generate icon.png if it doesn't exist using base64 (no Pillow required!)
    if not os.path.exists(f"{ext_dir}/icon.png"):
        print("[*] Generating extension icon...")
        icon_base64 = """iVBORw0KGgoAAAANSUhEUgAAAIAAAACACAIAAABMXPacAAADwElEQVR4nO2dvXLiQBCEx1eXcMkR
E/rBnPBcTvxgDhWTkV6gq60pGYRgd6e7V/NldrkA9zc7ox9q9Xb489cSHL/QH2DvpAAwKQBMCgCT
AsCkADApAEwKAJMCwKQAMCkATAoAkwLApAAwKQCMtoDrdLlOF/SnqOI3+gO8yJz758e3mZ2ni5kd
TkfkB3qVN7k7Yj56z/nr3QQ1KAm4F71HToOGgC3Re4Q0sAt4NnqPhAZeATXRe8g1MApoFb2HVgOX
gB7Rewg1sAjoHb2HSgNeQGT0HhINSAGo6D1wDRgBDNF7gBqiBbBF74FoiBPAHL0nWEOEAJXoPWEa
+gpQjN4ToKGXAPXoPV01tBcwUvSeThpaChg1ek9zDc0EXKfL2NF7GmrQvimPomGppQAwwgLmPnDv
RxVUv5YyIxq6R3UF3Ixe0YeqgGGQFLBS6XKLQFLASKQAMHoCHjYZrS6kJ2AwxARsrG6hRSAmYDzk
BXx+fEtfhVUS8FRjUelCSgKGREbAzYouzedmF5JYBDICRkVYwKLqRUexhoCXmwl/F9IQMDACAtbH
78Nfki8CAQFjIylgZd7KjWJ2AU0aCHMXYhcwPNQCto/fh39AuwioBW8B8QEbZ6zQKOYV0LxpcHYh
XgE7gfS7ofeqtbKKz1/vbN0pVwCYFACGUUDXack2ihkF7Ao6AQEVSrUISI+CFtQculDF/RO6FfCT
ygNHtuPOBVwCwqqVZ1lwCdghRAJeu/i8BeYL1EQC9gm1gIbzk3YUNxNwOB1rFjWkIdR836vVhikt
zwMOp+N59B1reLer8Ty7dU2/8dv2jRoWfqHLDKhsR5z0SN/6DeFKB52a2Msv2yl963otaONIgK+V
9dtkvTdO7HsYejgdpdvRXPhdt62MOA9YcRA2ftdf/N7mNwEbtwadiMmtg5j0LXjz7pV9LYueyGc4
3Hy74L2jQ2/IzP/V+dZZwv9n4gWuknt1MOzu6R7OTUbj0zfUxTjCkQBJ3/IRJoZ+ignFQ3yADlCF
X8DfDwC2I3j6xiDAQA4Y0jeGFlQIGwnwR1d5iATM9B4JJIVfoGhBnq7tiC19IxRg3RwQpm+ELajQ
cCRQNf0FvAJm6kcCZ+EXGFuQp/7bLszpG78Aq3DAn77xt6DCUyOBuekvkBEws2UkSBR+QaAFeR62
I630TU6APbrFr5W+ybWgwmIkCDX9BaoCZuaRoFj4BW0BZnadLrrp2wAC1NEbwoORAsCkADApAEwK
AJMCwKQAMCkATAoAkwLApAAwKQBMCgCTAsD8A9KvhT7hNewxAAAAAElFTkSuQmCC"""
        with open(f"{ext_dir}/icon.png", "wb") as f:
            f.write(base64.b64decode(icon_base64))
    
    # 2. Generate Private Key if not exists
    pem_path = "/home/cyber/CODES/aurablock/extension_key.pem"
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
    crx_path = f"/home/cyber/CODES/aurablock/extension.crx"
    if os.path.exists(crx_path):
        os.remove(crx_path)
        
    # Pack using Google Chrome CLI
    subprocess.run(f"google-chrome-stable --pack-extension={ext_dir} --pack-extension-key={pem_path} --no-sandbox", shell=True, check=True)
    
    # Move the crx file to build dir
    os.rename("/home/cyber/CODES/aurablock/extension.crx", f"{temp_dist_dir}/aurablock-shield.crx")
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
    with open("/home/cyber/CODES/aurablock/extension_build/ext_id.txt", "w") as f:
        f.write(ext_id)
        
    print("[+] Pre-build complete! Ready for installation script.")

if __name__ == "__main__":
    main()
