cmake_minimum_required(VERSION 3.21)
project(cpp)
set(CMAKE_CXX_STANDARD 20)

include(auto.cmake)
use_gtest()
autobuild()
