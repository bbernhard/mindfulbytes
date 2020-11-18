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
import shutil
import traceback

ImageFile.LOAD_TRUNCATED_IMAGES = True
EXTENSIONS = {'.jpg', '.png', 'jpeg'}

TOPIC = "imgreader-fs"


def get_redis_host():
    redis_address = os.getenv("REDIS_ADDRESS")
    if redis_address is None:
        return "localhost"
    
    redis_address = redis_address.split(":")
    if len(redis_address) != 2:
        print("Invalid Redis Address:%s" %redis_address, file=sys.stderr)
        sys.exit(1)
    return redis_address[0]

def get_redis_port():
    redis_address = os.getenv("REDIS_ADDRESS")
    if redis_address is None:
        return 6379
    
    redis_address = redis_address.split(":")
    if len(redis_address) != 2:
        print("Invalid Redis Address:%s" %redis_address, file=sys.stderr)
        sys.exit(1)

    try:
        return int(redis_address[1])
    except:
        print("Invalid Redis Address:%s" %redis_address, file=sys.stderr)
        sys.exit(1)

def fetch(id, destination):
    try:
        redis_client = redis.Redis(host=get_redis_host(), port=get_redis_port(), db=0)
        res = redis_client.get(TOPIC+":image:"+id)
        filename = res.decode("utf-8")
        shutil.copyfile(filename, destination)
    except:
        traceback.print_exc()
        print("Couldn't fetch data for id " + id, file=sys.stderr)
        sys.exit(1)

def crawl(directory):
    if not os.path.exists(directory):
        print("%s doesn't exist!" %directory)
        sys.exit(1)

    redis_client = redis.Redis(host=get_redis_host(), port=get_redis_port(), db=0)

    print("Delete existing entries")
    keys_to_delete = redis_client.keys(TOPIC+":*")
    for key in keys_to_delete:
        redis_client.delete(key)

    images_per_date = {}
    images_per_fulldate = {}
    for filename in Path(directory).rglob("*"):
        if filename.suffix.lower() in EXTENSIONS:
            print("Processing file %s" %filename)

            try:
                img = Image.open(filename)
            except:
                continue
            exif_data = img._getexif()
            if exif_data is not None:
                try:
                    datetime_str = exif_data[36867]
                    datetime_str = datetime_str.strip(" ")
                    if datetime_str != "":
                        

                        try:
                            d = datetime.strptime(datetime_str,"%Y:%m:%d %H:%M:%S")
                        except ValueError:
                            print("ERROR: Couldn't extract date from %s. INVALID FORMAT: %s" %(filename, datetime_str))
                            continue

                        u = str(uuid.uuid4())
                        date = d.date().strftime("%m-%d")
                        entry = {"uri": str(filename), "uuid": u}
                        try: 
                            imgs = images_per_date[date]
                            imgs.append(entry)
                            images_per_date[date] = imgs
                        except KeyError:
                            images_per_date[date] = [entry]

                        fulldate = d.date().strftime("%Y-%m-%d")
                        try: 
                            imgs = images_per_fulldate[fulldate]
                            imgs.append(entry)
                            images_per_fulldate[fulldate] = imgs
                        except KeyError:
                            images_per_fulldate[fulldate] = [entry]
                             
                        redis_client.set(TOPIC+":image:" + u, str(filename))
                except KeyError:
                    pass

    for key, value in images_per_date.items():
        redis_client.set(TOPIC+":date:" + key, json.dumps(value))

    for key, value in images_per_fulldate.items():
        redis_client.set(TOPIC+":fulldate:" + key, json.dumps(value))
    
    #redis_client.set("imgreader:date:" + d.date().strftime("%Y-%m-%d"), json.dumps(entry))

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Image Reader") 

    subparsers = parser.add_subparsers(help="", dest="command")
    
    crawl_subparser = subparsers.add_parser("crawl", help="Crawl")
    crawl_subparser.add_argument('--directory', type=str, help='Path to the image directory', required=True)

    fetch_subparser = subparsers.add_parser("fetch", help="Fetch")
    fetch_subparser.add_argument('--id', type=str, help='id', required=True)
    fetch_subparser.add_argument('--destination', type=str, help='destination', required=True)
    
    args = parser.parse_args() 

    if args.command != "crawl" and args.command != "fetch":
        print("Please specify either 'crawl' or 'fetch' as command", file=sys.stderr)
        sys.exit(1)

    if args.command == "crawl":
        crawl(args.directory)
    elif args.command == "fetch":
        fetch(args.id, args.destination)
    else:
        print("Unknown command", file=sys.stderr)
        sys.exit(1)


