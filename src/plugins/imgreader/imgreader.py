#!/usr/bin/env python3

from PIL import Image
from PIL import ImageFile
import sys
from pathlib import Path
from datetime import datetime
import hashlib
import argparse
import os
import redis
import uuid
import json

ImageFile.LOAD_TRUNCATED_IMAGES = True
EXTENSIONS = {'.jpg', '.png', 'jpeg'}

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Image Reader") 
    parser.add_argument('--directory', type=str, help='Path to the image directory', required=True)
    args = parser.parse_args()

    if not os.path.exists(args.directory):
        print("%s doesn't exist!" %args.directory)
        sys.exit(1)

    redis_client = redis.Redis(host='localhost', port=6379, db=0)
    pub_sub_client = redis_client.pubsub()

    images = {}
    for filename in Path(args.directory).rglob("*"):
        if filename.suffix.lower() in EXTENSIONS:
            print(filename)

            try:
                img = Image.open(filename)
            except OSError:
                pass 
            exif_data = img._getexif()
            if exif_data is not None:
                try:
                    datetime_str = exif_data[36867]
                    datetime_str = datetime_str.strip(" ")
                    if datetime_str != "":
                        d = datetime.strptime(datetime_str,"%Y:%m:%d %H:%M:%S")
                        
                        u = str(uuid.uuid4())
                        
                        print("%s %s" %(d.date().strftime("%Y-%m-%d"), u))
                        
                        k = d.date().strftime("%m-%d")
                        entry = {"filename": str(filename), "uuid": u}
                        try: 
                            imgs = images[k]
                            imgs.append(entry)
                            images[k] = imgs
                        except KeyError:
                            images[k] = [entry]
                            

                        redis_client.set("imgreader:date:" + d.date().strftime("%Y-%m-%d"), json.dumps(entry)) 
                        redis_client.set("imgreader:image:" + u, str(filename))
                except KeyError:
                    pass

    for key, value in images.items():
        redis_client.set("imgreader:day:" + k, json.dumps(value)) 


