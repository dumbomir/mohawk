%global provider        github
%global provider_tld    com
%global project         yaacov
%global repo            mohawk
%global provider_prefix %{provider}.%{provider_tld}/%{project}/%{repo}
%global import_path     %{provider_prefix}
%global commit          8eefab3f90ab8c828e202a5a0fc20150ecae1ff2
%global shortcommit     %(c=%{commit}; echo ${c:0:7})

Name:           %{repo}
Version:        0.26.2
Release:        7%{?dist}
Summary:        Time series metric data storage
License:        Apache
URL:            https://%{import_path}
Source0:        https://github.com/MohawkTSDB/mohawk/archive/%{version}.tar.gz

BuildRequires:  gcc
BuildRequires:  bzr
BuildRequires:  git

BuildRequires:  golang >= 1.2-7
BuildRequires:  golang-github-mattn-go-sqlite3-devel
BuildRequires:  golang-github-go-mgo-mgo-devel
BuildRequires:  golang-github-spf13-cobra-devel
BuildRequires:  golang-github-spf13-viper-devel

%description
Mohawk is a metric data storage engine that uses a plugin architecture for data storage and a simple REST API as the primary interface.

%prep
%setup -q -n mohawk-%{version}

# many golang binaries are "vendoring" (bundling) sources, so remove them. Those dependencies need to be packaged independently.
rm -rf vendor

%build
# set up temporary build gopath, and put our directory there
mkdir -p ./_build/src/github.com/MohawkTSDB
ln -s $(pwd) ./_build/src/github.com/MohawkTSDB/mohawk

export GOPATH=$(pwd)/_build:%{gopath}
make

%install
install -d %{buildroot}%{_bindir}
install -p -m 0755 ./mohawk %{buildroot}%{_bindir}/mohawk

%files
%defattr(-,root,root,-)
%doc LICENSE README.md
%{_bindir}/mohawk

%changelog
* Wed Dec 6 2017 Yaacov Zamir <kobi.zamir@gmail.com> 0.26.2-7
- Initial RPM release
