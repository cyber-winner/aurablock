import undetected_chromedriver as uc
import time
import sys
import os

options = uc.ChromeOptions()
options.add_argument('--headless')
options.add_argument(f'--load-extension={os.path.abspath("extension")}')

try:
    driver = uc.Chrome(options=options)
    driver.get('chrome://extensions')
    print("Successfully started Chrome with the extension.")
    driver.quit()
except Exception as e:
    print(f"Error loading extension: {e}")
