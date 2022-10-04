for r in `grep rec_ /etc/rendezvous/rendezvous.conf` 
do 
   #echo $r 
   file="/etc/rendezvous/$r"
   #echo "@@ $file"
   if [ -f $file ] 
   then
      addr=$(grep address $file | awk -F ' ' '{print $2}')
      #echo "\"$file\" => \"$addr\""
      echo "insert into Selfie_Host (RendServerIp, Host, SipAddr) values (1566400727, \"$r\", \"$addr\");"
   fi
done
