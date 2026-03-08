package entity

import (
	"errors"
	"net"
)

// Domain errors
var (
	ErrInvalidIP   = errors.New("invalid IP address")
	ErrInvalidCIDR = errors.New("invalid CIDR notation")
)

// IPValueObject представляет значение IP-адреса
type IPValueObject struct {
	address string
}

// NewIPValueObject создает новый объект IP
func NewIPValueObject(address string) (*IPValueObject, error) {
	ip := net.ParseIP(address)
	if ip == nil {
		return nil, ErrInvalidIP
	}
	return &IPValueObject{address: address}, nil
}

// String возвращает строковое представление IP
func (ip *IPValueObject) String() string {
	return ip.address
}

// SubnetEntity представляет доверенную подсеть
type SubnetEntity struct {
	Cidr  string
	ipNet *net.IPNet
}

// NewSubnetEntity создает новую подсеть
func NewSubnetEntity(cidr string) (*SubnetEntity, error) {
	if cidr == "" {
		return &SubnetEntity{Cidr: "", ipNet: nil}, nil
	}

	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, ErrInvalidCIDR
	}

	return &SubnetEntity{
		Cidr:  cidr,
		ipNet: ipNet,
	}, nil
}

// Contains проверяет, содержит ли подсеть IP
func (s *SubnetEntity) Contains(ip *IPValueObject) bool {
	if s.ipNet == nil {
		return false
	}
	return s.ipNet.Contains(net.ParseIP(ip.address))
}

// IsEmpty проверяет, пустая ли подсеть
func (s *SubnetEntity) IsEmpty() bool {
	return s.Cidr == ""
}

// String возвращает строковое представление подсети
func (s *SubnetEntity) String() string {
	if s.IsEmpty() {
		return "not configured"
	}
	return s.Cidr
}
