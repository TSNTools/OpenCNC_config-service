#!/usr/bin/env python3
import sys
import csv
import re
import xml.dom.minidom
import xml.etree.ElementTree as ET
from ncclient import manager
from ncclient.operations import RPCError

# ==========================================
# SWITCH CONNECTION PARAMETERS
# ==========================================
HOST = "192.168.4.64"
PORT = 830
USER = "sys-admin"  # Ensure this is correct
PASS = "sys-admin"  # Ensure this is correct

def pretty_xml(xml_str):
    """Parses and formats XML to be human-readable."""
    try:
        dom = xml.dom.minidom.parseString(xml_str)
        return dom.toprettyxml(indent="  ")
    except Exception:
        return xml_str

def remove_namespaces(xml_string):
    """Strips namespaces from XML tags so it's easier to parse into a CSV."""
    # Remove xmlns="..."
    xml_string = re.sub(r'\sxmlns="[^"]+"', '', xml_string, count=0)
    # Remove xmlns:prefix="..."
    xml_string = re.sub(r'\sxmlns:[^="]+="[^"]+"', '', xml_string, count=0)
    # Remove namespace prefixes from tags (e.g., <dot1q:bridge-type> -> <bridge-type>)
    xml_string = re.sub(r'<\/?(\w+):', lambda m: m.group(0).replace(m.group(1)+':', ''), xml_string)
    return xml_string

def main():
    print(f"Connecting to {HOST}:{PORT}...")
    try:
        with manager.connect(
            host=HOST,
            port=PORT,
            username=USER,
            password=PASS,
            hostkey_verify=False,
            device_params={"name": "default"},
            timeout=30,
        ) as m:
            print(f"[+] Connected successfully to {HOST}\n")

            bridge_filter = """
            <filter xmlns="urn:ietf:params:xml:ns:netconf:base:1.0">
              <bridges xmlns="urn:ieee:std:802.1Q:yang:ieee802-dot1q-bridge"/>
            </filter>
            """

            print("[*] Fetching running bridge configuration...")
            reply = m.get_config(source="running", filter=bridge_filter)
            
            raw_xml = reply.data_xml
            pretty = pretty_xml(raw_xml)
            
            # ==========================================
            # 1. Save Pretty XML File
            # ==========================================
            xml_filename = "bridge_config.xml"
            with open(xml_filename, "w", encoding="utf-8") as f:
                f.write(pretty)
            print(f"[+] SUCCESS: Saved raw pretty-printed configuration to '{xml_filename}'")

            # ==========================================
            # 2. Parse and Save CSV File
            # ==========================================
            csv_filename = "bridge_config.csv"
            clean_xml = remove_namespaces(raw_xml)
            root = ET.fromstring(clean_xml)

            with open(csv_filename, "w", newline="", encoding="utf-8") as csvfile:
                writer = csv.writer(csvfile)
                # Write the CSV Header Row
                writer.writerow(["Bridge Name", "Bridge Type", "Component Name", "VLAN ID", "VLAN Name"])

                # Find all bridges in the XML
                bridges = root.findall(".//bridge")
                
                if not bridges:
                    print("[-] No bridges found in the configuration to write to CSV.")
                
                for bridge in bridges:
                    bridge_name = bridge.findtext("name", default="N/A")
                    bridge_type = bridge.findtext("bridge-type", default="N/A")
                    
                    components = bridge.findall(".//component")
                    if not components:
                        writer.writerow([bridge_name, bridge_type, "N/A", "N/A", "N/A"])
                        continue
                        
                    for comp in components:
                        comp_name = comp.findtext("name", default="N/A")
                        
                        vlans = comp.findall(".//vlan")
                        if not vlans:
                            # Write the row even if there are no VLANs configured yet
                            writer.writerow([bridge_name, bridge_type, comp_name, "None", "None"])
                        
                        for vlan in vlans:
                            vid = vlan.findtext("vid", default="N/A")
                            vlan_name = vlan.findtext("name", default="N/A")
                            writer.writerow([bridge_name, bridge_type, comp_name, vid, vlan_name])

            print(f"[+] SUCCESS: Extracted bridge details and saved to '{csv_filename}'")

    except RPCError as e:
        print(f"[-] NETCONF RPC Error: {e.message}")
    except Exception as err:
        print(f"[!] Connection or execution failed: {err}")
        sys.exit(1)

if __name__ == "__main__":
    main()