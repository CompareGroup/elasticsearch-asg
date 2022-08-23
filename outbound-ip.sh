# enter container name
echo enter container name
read container

#  enter outbound ip you want to assign the container
echo enter outbound nat ip to set
read nat_ip

# get ip of container and store as $container_ip
container_ip=$(docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $container)

# add nat rule to postrouting table
sudo iptables -t nat -I POSTROUTING -p all -s $container_ip/32 -j SNAT --to-source $nat_ip

# verify rule has been added
sudo iptables -t nat -v -L POSTROUTING -n --line-number | grep $container_ip
